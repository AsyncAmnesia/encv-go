#ifndef LIBSWSCALE_SWSCALE_H
#define LIBSWSCALE_SWSCALE_H

#include <stdint.h>

struct SwsContext;

#define SWS_BICUBIC 4

enum AVPixelFormat {
    AV_PIX_FMT_BGR0 = 28,
    AV_PIX_FMT_RGB32 = 25,
};

struct SwsContext *sws_getContext(int srcW, int srcH, enum AVPixelFormat srcFormat,
                                   int dstW, int dstH, enum AVPixelFormat dstFormat,
                                   int flags, void *srcFilter, void *dstFilter,
                                   const double *param);
int sws_scale(struct SwsContext *c, const uint8_t *const srcSlice[], const int srcStride[],
               int srcSliceY, int srcSliceH, uint8_t *const dstSlice[], const int dstStride[]);
void sws_freeContext(struct SwsContext *swsContext);

#endif
