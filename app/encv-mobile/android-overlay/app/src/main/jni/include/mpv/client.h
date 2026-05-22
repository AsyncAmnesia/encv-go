/* Copyright (C) 2017 the mpv developers
 * Permission to use, copy, modify, and/or distribute this software for any
 * purpose with or without fee is hereby granted.
 * SPDX-License-Identifier: ISC */
#ifndef MPV_CLIENT_API_H_
#define MPV_CLIENT_API_H_
#include <stddef.h>
#include <stdint.h>
#ifdef _WIN32
#define MPV_EXPORT __declspec(dllexport)
#elif defined(__GNUC__) || defined(__clang__)
#define MPV_EXPORT __attribute__((visibility("default")))
#else
#define MPV_EXPORT
#endif
#ifdef __cplusplus
extern "C" {
#endif

#define MPV_MAKE_VERSION(major, minor) (((major) << 16) | (minor) | 0UL)
#define MPV_CLIENT_API_VERSION MPV_MAKE_VERSION(2, 3)

MPV_EXPORT unsigned long mpv_client_api_version(void);

typedef struct mpv_handle mpv_handle;

typedef enum mpv_error {
    MPV_ERROR_SUCCESS           = 0,
    MPV_ERROR_EVENT_QUEUE_FULL  = -1,
    MPV_ERROR_NOMEM             = -2,
    MPV_ERROR_UNINITIALIZED     = -3,
    MPV_ERROR_INVALID_PARAMETER = -4,
    MPV_ERROR_OPTION_NOT_FOUND  = -5,
    MPV_ERROR_OPTION_FORMAT     = -6,
    MPV_ERROR_OPTION_ERROR      = -7,
    MPV_ERROR_PROPERTY_NOT_FOUND = -8,
    MPV_ERROR_PROPERTY_FORMAT   = -9,
    MPV_ERROR_PROPERTY_UNAVAILABLE = -10,
    MPV_ERROR_PROPERTY_ERROR    = -11,
    MPV_ERROR_COMMAND           = -12,
    MPV_ERROR_LOADING_FAILED    = -13,
    MPV_ERROR_AO_INIT_FAILED    = -14,
    MPV_ERROR_VO_INIT_FAILED    = -15,
    MPV_ERROR_NOTHING_TO_PLAY   = -16,
    MPV_ERROR_UNKNOWN_FORMAT    = -17,
    MPV_ERROR_UNSUPPORTED       = -18,
    MPV_ERROR_NOT_IMPLEMENTED   = -19,
    MPV_ERROR_GENERIC           = -20
} mpv_error;

MPV_EXPORT const char *mpv_error_string(int error);
MPV_EXPORT void mpv_free(void *data);

enum mpv_log_level {
    MPV_LOG_LEVEL_NONE   = 0,
    MPV_LOG_LEVEL_FATAL  = 10,
    MPV_LOG_LEVEL_ERROR  = 20,
    MPV_LOG_LEVEL_WARN   = 30,
    MPV_LOG_LEVEL_INFO   = 40,
    MPV_LOG_LEVEL_V      = 50,
    MPV_LOG_LEVEL_DEBUG  = 60,
    MPV_LOG_LEVEL_TRACE  = 70,
};

typedef enum mpv_format {
    MPV_FORMAT_NONE             = 0,
    MPV_FORMAT_STRING           = 1,
    MPV_FORMAT_OSD_STRING       = 2,
    MPV_FORMAT_FLAG             = 3,
    MPV_FORMAT_INT64            = 4,
    MPV_FORMAT_DOUBLE           = 5,
    MPV_FORMAT_NODE             = 6,
    MPV_FORMAT_NODE_ARRAY       = 7,
    MPV_FORMAT_NODE_MAP         = 8,
    MPV_FORMAT_BYTE_ARRAY       = 9
} mpv_format;

typedef enum mpv_event_id {
    MPV_EVENT_NONE                    = 0,
    MPV_EVENT_SHUTDOWN                = 1,
    MPV_EVENT_LOG_MESSAGE             = 2,
    MPV_EVENT_GET_PROPERTY_REPLY      = 3,
    MPV_EVENT_SET_PROPERTY_REPLY      = 4,
    MPV_EVENT_COMMAND_REPLY           = 5,
    MPV_EVENT_START_FILE              = 6,
    MPV_EVENT_END_FILE                = 7,
    MPV_EVENT_FILE_LOADED            = 8,
    MPV_EVENT_TRACKS_CHANGED          = 9,
    MPV_EVENT_TRACK_SWITCHED          = 10,
    MPV_EVENT_IDLE                   = 11,
    MPV_EVENT_PAUSE                  = 12,
    MPV_EVENT_UNPAUSE                = 13,
    MPV_EVENT_TICK                   = 14,
    MPV_EVENT_SCRIPT_INPUT_DISPATCH  = 15,
    MPV_EVENT_CLIENT_MESSAGE          = 16,
    MPV_EVENT_VIDEO_RECONFIG          = 17,
    MPV_EVENT_AUDIO_RECONFIG          = 18,
    MPV_EVENT_METADATA_UPDATE         = 19,
    MPV_EVENT_SEEK                   = 20,
    MPV_EVENT_PLAYBACK_RESTART        = 21,
    MPV_EVENT_PROPERTY_CHANGE        = 22,
    MPV_EVENT_CHAPTER_CHANGE          = 23,
    MPV_EVENT_QUEUE_OVERFLOW          = 24,
} mpv_event_id;

typedef enum mpv_end_file_reason {
    MPV_END_FILE_REASON_EOF = 0,
    MPV_END_FILE_REASON_STOP = 2,
    MPV_END_FILE_REASON_QUIT = 3,
    MPV_END_FILE_REASON_ERROR = 4,
    MPV_END_FILE_REASON_REDIRECT = 5,
} mpv_end_file_reason;

struct mpv_event;
struct mpv_event_property;

typedef struct mpv_event {
    mpv_event_id event_id;
    int error;
    unsigned long reply_userdata;
    void *data;
} mpv_event;

typedef struct mpv_event_property {
    char *name;
    mpv_format format;
    void *data;
} mpv_event_property;

typedef struct mpv_event_log_message {
    char *prefix;
    int level;
    enum mpv_log_level log_level;
    char *text;
    int *log_level_internal;
} mpv_event_log_message;

typedef struct mpv_event_end_file {
    enum mpv_end_file_reason reason;
    int error;
    mpv_error reserved_for_future_use1;
} mpv_event_end_file;

typedef struct mpv_byte_array {
    void *data;
    size_t size;
} mpv_byte_array;

typedef struct mpv_node_list {
    int num;
    char **keys;
    struct mpv_node *values;
} mpv_node_list;

typedef struct mpv_node {
    mpv_format format;
    union {
        long long int64;
        double double_;
        char *string;
        struct mpv_node_list *list;
        struct mpv_byte_array *ba;
    } u;
    struct mpv_byte_array *ba;
} mpv_node;

MPV_EXPORT const char *mpv_event_name(mpv_event_id event);

MPV_EXPORT mpv_handle *mpv_create(void);
MPV_EXPORT int mpv_initialize(mpv_handle *ctx);
MPV_EXPORT void mpv_terminate_destroy(mpv_handle *ctx);
MPV_EXPORT void mpv_detach_destroy(mpv_handle *ctx);

MPV_EXPORT int mpv_load_config_file(mpv_handle *ctx, const char *filename);
MPV_EXPORT int mpv_load_config_files(mpv_handle *ctx);

MPV_EXPORT const char **mpv_get_property_string_list(mpv_handle *ctx, const char *name);
MPV_EXPORT void mpv_free(void *data);

MPV_EXPORT int mpv_set_option_string(mpv_handle *ctx, const char *name, const char *data);
MPV_EXPORT int mpv_set_option(mpv_handle *ctx, const char *name, mpv_format format, void *data);
MPV_EXPORT int mpv_command(mpv_handle *ctx, const char **args);
MPV_EXPORT int mpv_command_node(mpv_handle *ctx, mpv_node *args, mpv_node *result);
MPV_EXPORT int mpv_command_async(mpv_handle *ctx, unsigned long reply_userdata, const char **args);
MPV_EXPORT int mpv_command_node_async(mpv_handle *ctx, unsigned long reply_userdata, mpv_node *args);

MPV_EXPORT int mpv_set_property(mpv_handle *ctx, const char *name, mpv_format format, void *data);
MPV_EXPORT int mpv_set_property_string(mpv_handle *ctx, const char *name, const char *data);
MPV_EXPORT int mpv_set_property_async(mpv_handle *ctx, unsigned long userdata, const char *name, mpv_format format, void *data);

MPV_EXPORT int mpv_get_property(mpv_handle *ctx, const char *name, mpv_format format, void *data);
MPV_EXPORT char *mpv_get_property_string(mpv_handle *ctx, const char *name);
MPV_EXPORT char *mpv_get_property_osd_string(mpv_handle *ctx, const char *name);
MPV_EXPORT int mpv_get_property_async(mpv_handle *ctx, unsigned long userdata, const char *name, mpv_format format);

MPV_EXPORT int mpv_observe_property(mpv_handle *ctx, unsigned long registered_reply_userdata, const char *name, mpv_format format);
MPV_EXPORT int mpv_unobserve_property(mpv_handle *ctx, unsigned long registered_reply_userdata, int count);

MPV_EXPORT mpv_event *mpv_wait_event(mpv_handle *ctx, double timeout);
MPV_EXPORT void mpv_wakeup(mpv_handle *ctx);
MPV_EXPORT void mpv_set_wakeup_callback(mpv_handle *ctx, void (*cb)(void *d), void *d);
MPV_EXPORT unsigned long mpv_hook_add(mpv_handle *ctx, const char *name, unsigned long user_data, int priority);
MPV_EXPORT void mpv_hook_continue(mpv_handle *ctx, unsigned long id);

MPV_EXPORT int mpv_request_log_messages(mpv_handle *ctx, const char *min_level);

MPV_EXPORT void mpv_free_node_contents(mpv_node *node);

#ifdef __cplusplus
}
#endif
#endif
