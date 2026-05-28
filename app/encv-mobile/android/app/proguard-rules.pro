# Bugly 混淆保留
-dontwarn com.tencent.bugly.**
-keep public class com.tencent.bugly.**{*;}

# ComboLite: 保留所有类和 @Metadata 注解
# 框架内部使用 ::function.javaMethod (kotlin-reflect) 做权限检查
# R8 会剥离/修改 @Metadata 导致 kotlin-reflect 无法解析函数签名
# ComboLite demo 未启用 R8 (isMinifyEnabled=false)，其 consumer-rules.pro 不完整
# 受影响类: PluginManager, InstallerManager, PluginCrashHandler, AuthorizationManager
-keep class com.combo.core.** { *; }
-keep interface com.combo.core.** { *; }

# 保留 Kotlin @Metadata 注解（kotlin-reflect 解析函数签名必需）
-keepattributes RuntimeVisibleAnnotations,RuntimeInvisibleAnnotations

# kotlin-reflect: 防止 R8 混淆内部类导致反射失败
-keep class kotlin.reflect.** { *; }
-dontwarn kotlin.reflect.**
