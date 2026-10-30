import 'package:flutter/material.dart';

import '../api/api_client.dart';
import 'html_viewer_stub.dart' if (dart.library.html) 'html_viewer_web.dart';

/// 证书查看 / 打印页（本人证书 HTML，可直接另存 / 打印为 PDF）。
class CertViewerPage extends StatefulWidget {
  const CertViewerPage({super.key, required this.certNo, this.api});

  final String certNo;
  final ApiClient? api;

  @override
  State<CertViewerPage> createState() => _CertViewerPageState();
}

class _CertViewerPageState extends State<CertViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<String> _future = _api.certHtml(widget.certNo);

  void _reload() {
    setState(() => _future = _api.certHtml(widget.certNo));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.certNo),
        actions: <Widget>[
          IconButton(
            tooltip: '刷新',
            onPressed: _reload,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: FutureBuilder<String>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<String> snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError || (snap.data ?? '').isEmpty) {
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('证书加载失败（已撤销证书不可下载）'),
                  const SizedBox(height: 8),
                  FilledButton(onPressed: _reload, child: const Text('重试')),
                ],
              ),
            );
          }
          return HtmlViewer(html: snap.data!);
        },
      ),
    );
  }
}
