import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'html_viewer_stub.dart'
    if (dart.library.html) 'html_viewer_web.dart';

/// 资源阅读页：文本类资源端内展示。
///
/// - markdown → 渲染 Markdown（讲义）
/// - html → iframe（Web）/ WebView（Android）渲染
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
              return HtmlViewer(html: content);
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
