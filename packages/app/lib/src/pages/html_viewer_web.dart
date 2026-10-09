// ignore_for_file: avoid_web_libraries_in_flutter
// Web 专属文件：dart:html 仅用于 iframe 渲染，条件导入保证非 Web 不编译本文件。
import 'dart:html' as dhtml;
import 'dart:ui_web' as ui_web;

import 'package:flutter/widgets.dart';

/// Web 平台 HTML 渲染：iframe + srcdoc，最忠实还原富文本页面。
///
/// webview_flutter 官方包不支持 Web 平台，故 Web 端用原生 iframe 渲染；
/// Android/iOS 走 html_viewer_stub.dart（webview_flutter）。
class HtmlViewer extends StatelessWidget {
  const HtmlViewer({super.key, required this.html});

  final String html;

  @override
  Widget build(BuildContext context) {
    final String viewType = 'html-iframe-${identityHashCode(this)}';
    ui_web.platformViewRegistry.registerViewFactory(viewType, (int viewId) {
      final dhtml.IFrameElement iframe = dhtml.IFrameElement()
        ..srcdoc = html
        ..style.width = '100%'
        ..style.height = '100%'
        ..style.border = 'none';
      return iframe;
    });
    return HtmlElementView(viewType: viewType);
  }
}
