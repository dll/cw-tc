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
  Widget build(BuildContext context) {
    return WebViewWidget(controller: _controller);
  }
}
