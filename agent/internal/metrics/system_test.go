package metrics

import (
	"encoding/json"
	"testing"

	"github.com/shirou/gopsutil/v4/sensors"
)

func TestSelectCPUTemperature(t *testing.T) {
	tests := []struct {
		name   string
		temps  []sensors.TemperatureStat
		want   *float64
		wantV  float64
	}{
		{
			name:  "empty input returns nil",
			temps: nil,
			want:  nil,
		},
		{
			name: "Intel package preferred over core",
			temps: []sensors.TemperatureStat{
				{SensorKey: "coretemp_core_0", Temperature: 55},
				{SensorKey: "coretemp_packageid0", Temperature: 58},
				{SensorKey: "coretemp_core_1", Temperature: 53},
			},
			wantV: 58,
		},
		{
			name: "AMD tdie preferred over tctl",
			temps: []sensors.TemperatureStat{
				{SensorKey: "k10temp_tctl", Temperature: 60},
				{SensorKey: "k10temp_tdie", Temperature: 50},
			},
			wantV: 50,
		},
		{
			name: "tctl preferred over generic core",
			temps: []sensors.TemperatureStat{
				{SensorKey: "coretemp_core_0", Temperature: 55},
				{SensorKey: "k10temp_tctl", Temperature: 60},
			},
			wantV: 60,
		},
		{
			name: "Raspberry Pi cpu_thermal",
			temps: []sensors.TemperatureStat{
				{SensorKey: "cpu_thermal", Temperature: 45},
			},
			wantV: 45,
		},
		{
			name: "ARM cpu-thermal variant",
			temps: []sensors.TemperatureStat{
				{SensorKey: "cpu-thermal", Temperature: 42},
			},
			wantV: 42,
		},
		{
			name: "core sensor at priority 2",
			temps: []sensors.TemperatureStat{
				{SensorKey: "acpitz", Temperature: 30},
				{SensorKey: "coretemp_core_0", Temperature: 55},
			},
			wantV: 55,
		},
		{
			name: "fallback to first valid sensor",
			temps: []sensors.TemperatureStat{
				{SensorKey: "acpitz", Temperature: 30},
			},
			wantV: 30,
		},
		{
			name: "skip zero temperature",
			temps: []sensors.TemperatureStat{
				{SensorKey: "coretemp_core_0", Temperature: 0},
			},
			want: nil,
		},
		{
			name: "skip negative temperature",
			temps: []sensors.TemperatureStat{
				{SensorKey: "coretemp_core_0", Temperature: -5},
			},
			want: nil,
		},
		{
			name: "skip over 150°C",
			temps: []sensors.TemperatureStat{
				{SensorKey: "coretemp_core_0", Temperature: 200},
			},
			want: nil,
		},
		{
			name: "skip invalid, pick valid",
			temps: []sensors.TemperatureStat{
				{SensorKey: "coretemp_core_0", Temperature: -1},
				{SensorKey: "coretemp_core_1", Temperature: 55},
			},
			wantV: 55,
		},
		{
			name: "case insensitive matching",
			temps: []sensors.TemperatureStat{
				{SensorKey: "CORETEMP_PACKAGEID0", Temperature: 62},
			},
			wantV: 62,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectCPUTemperature(tt.temps)

			if tt.want == nil && tt.wantV == 0 {
				if got != nil {
					t.Errorf("expected nil, got %v", *got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected non-nil result, got nil")
			}
			if *got != tt.wantV {
				t.Errorf("expected %v, got %v", tt.wantV, *got)
			}
		})
	}
}

func TestCPUMetrics_JSON_Omitempty(t *testing.T) {
	t.Run("nil temperature omitted from JSON", func(t *testing.T) {
		m := CPUMetrics{UsagePercent: 42.5, Cores: 4}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if contains(s, "temperature_celsius") {
			t.Errorf("expected temperature_celsius to be omitted, got: %s", s)
		}
	})

	t.Run("non-nil temperature present in JSON", func(t *testing.T) {
		temp := 58.0
		m := CPUMetrics{UsagePercent: 42.5, Cores: 4, TemperatureCelsius: &temp}
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if !contains(s, `"temperature_celsius":58`) {
			t.Errorf("expected temperature_celsius in JSON, got: %s", s)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
