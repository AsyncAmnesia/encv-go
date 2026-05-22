#ifndef LIBAVCODEC_JNI_H
#define LIBAVCODEC_JNI_H

#include <jni.h>

#ifdef __cplusplus
extern "C" {
#endif

int av_jni_set_java_vm(void *vm, void *log_ctx);
int av_jni_set_android_app_ctx(void *app_ctx, void *log_ctx);

#ifdef __cplusplus
}
#endif

#endif
