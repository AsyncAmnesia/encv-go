# Keep all Capacitor plugins from being removed by R8
-keep public class * extends com.getcapacitor.Plugin { *; }

# Keep local classes
-keep class com.encvgo.app.** { *; }
