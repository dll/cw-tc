import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';

/// 资源阅读页：文本类资源端内展示。
///
/// - markdown → 渲染 Markdown（讲义）
/// - html → WebView 渲染（HTML 课件）
/// - code → 纯文本（可复制）
class ResourceViewerPage extends StatefulWidget {
  const ResourceViewerPage({super.key, required this.resource, this.api});

  final LectureResource resource;
  final ApiClient? api;

  @override
  State<ResourceViewerPage> createState() => _ResourceViewerPageState();
}

class _ResourceViewerPageState extends State<ResourceViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<String> _future;

  @override
  void initState() {
    super.initState();
    _future = _api.fetchResourceText(widget.resource);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.resource.fileName)),
      body: FutureBuilder<String>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<String> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('资源加载失败'),
                  const SizedBox(height: 8),
                  FilledButton(
                    onPressed: () => setState(() {
                      _future = _api.fetchResourceText(widget.resource);
                    }),
                    child: const Text('重试'),
                  ),
                ],
              ),
            );
          }
          final content = snapshot.data ?? '';
          switch (widget.resource.kind) {
            case 'markdown':
              return Markdown(data: content, selectable: true);
            case 'html':
              return _HtmlViewer(html: content);
            default:
              return SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  content,
                  style: const TextStyle(fontSize: 15, height: 1.6),
                ),
              );
          }
        },
      ),
    );
  }
}

/// HTML 资源用 WebView 渲染（保持原始排版/样式）。
class _HtmlViewer extends StatefulWidget {
  const _HtmlViewer({required this.html});

  final String html;

  @override
  State<_HtmlViewer> createState() => _HtmlViewerState();
}

class _HtmlViewerState extends State<_HtmlViewer> {
  late final WebViewController _controller;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..loadHtmlString(widget.html);
  }

  @override
  Widget build(BuildContext context) {
    return WebViewWidget(controller: _controller);
  }
}
