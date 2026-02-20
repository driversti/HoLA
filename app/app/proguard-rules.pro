# Retrofit
-keepattributes Signature
-keepattributes *Annotation*
-keep class dev.driversti.hola.data.model.** { *; }

# Kotlinx Serialization
-keepclassmembers class kotlinx.serialization.json.** { *; }
-keep,includedescriptorclasses class dev.driversti.hola.**$$serializer { *; }
-keepclassmembers class dev.driversti.hola.** {
    *** Companion;
}
