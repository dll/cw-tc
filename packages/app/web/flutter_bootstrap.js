{{flutter_js}}
{{flutter_build_config}}

// CanvasKit 默认从 www.gstatic.com 加载，国内网络不可达会导致整页白屏。
// 构建产物已自带 canvaskit/，这里显式指向同源目录，避免依赖境外 CDN。
_flutter.loader.load({
  config: {
    canvasKitBaseUrl: "canvaskit/",
  },
  serviceWorkerSettings: {
    serviceWorkerVersion: {{flutter_service_worker_version}},
  },
});
