import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'slide_renderer_stub.dart'
    if (dart.library.html) 'slide_renderer_web.dart';

/// 课件（PPTX）阅读页：拉取二进制为 base64，交给 pptxjs 在 WebView 内渲染。
///
/// 端内渲染（不可下载）满足防盗要求；pptxjs 由 jsDelivr CDN 加载。
class SlideViewerPage extends StatefulWidget {
  const SlideViewerPage({super.key, required this.resource, this.api});

  final LectureResource resource;
  final ApiClient? api;

  @override
  State<SlideViewerPage> createState() => _SlideViewerPageState();
}

class _SlideViewerPageState extends State<SlideViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<String> _future;

  @override
  void initState() {
    super.initState();
    _future = _api.fetchResourceBase64(widget.resource);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.resource.title)),
      body: FutureBuilder<String>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<String> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError || (snapshot.data ?? '').isEmpty) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('课件加载失败'),
                  const SizedBox(height: 8),
                  FilledButton(
                    onPressed: () => setState(() {
                      _future = _api.fetchResourceBase64(widget.resource);
                    }),
                    child: const Text('重试'),
                  ),
                ],
              ),
            );
          }
          return SlideRenderer(base64: snapshot.data!);
        },
      ),
    );
  }
}
