import 'package:flutter/widgets.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// 非 Web 平台（Android/iOS）HTML 渲染：webview_flutter。
///
/// Web 端走 html_viewer_web.dart（iframe + srcdoc）。
class HtmlViewer extends StatefulWidget {
  const HtmlViewer({super.key, required this.html});

  final String html;

  @override
  State<HtmlViewer> createState() => _HtmlViewerState();
}

class _HtmlViewerState extends State<HtmlViewer> {
  late final WebViewController _controller = WebViewController()
    ..setJavaScriptMode(JavaScriptMode.unrestricted)
    ..loadHtmlString(widget.html);

  @override
  void initState() {
    super.initState();
    // Android WebView 默认要求用户手势才允许媒体播放；关闭该限制，
    // 课件里的配音/视频在首次点击后即可有声续播。
    final platform = _controller.platform;
    try {
      (platform as dynamic).setMediaPlaybackRequiresUserGesture(false);
    } catch (_) {
      // iOS/其他平台无此设置，忽略。
    }
  }

  @override
  Widget build(BuildContext context) {
    return WebViewWidget(controller: _controller);
  }
}
