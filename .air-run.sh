#!/bin/sh
# 兜底：即使 pm2 / air 中间层丢 env，这里强制 export
# ApplyMobileOverlay 的触发条件 (internal/config/config.go:292-294)
# 必须在 exec 之前，否则 ./tmp/encv 进程看不到这两个 env。
# `:-1` 模式：外部已设时保留外部值（尊重用户），未设时填 1。
# desktop 端用户想走正常模式可显式 `unset ENCV_DEV_PREVIEW ENCV_MOBILE && air`。
export ENCV_DEV_PREVIEW="${ENCV_DEV_PREVIEW:-1}"
export ENCV_MOBILE="${ENCV_MOBILE:-1}"
exec ./tmp/encv start "$@"
