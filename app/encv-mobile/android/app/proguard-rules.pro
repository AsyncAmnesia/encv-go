# Keep all Capacitor plugins from being removed by R8
-keep public class * extends com.getcapacitor.Plugin { *; }

# Keep local classes
-keep class com.encvgo.app.** { *; }

# Keep Lynx SDK classes
-keep class com.lynx.** { *; }
-keep class org.lynxsdk.** { *; }

# Keep mpv native methods (JNI bridge requires exact class name)
-keep class is.xyz.mpv.** { *; }
