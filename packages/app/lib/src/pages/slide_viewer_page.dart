import 'package:flutter/material.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';

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
          return _SlideWebView(base64: snapshot.data!);
        },
      ),
    );
  }
}

class _SlideWebView extends StatefulWidget {
  const _SlideWebView({required this.base64});

  final String base64;

  @override
  State<_SlideWebView> createState() => _SlideWebViewState();
}

class _SlideWebViewState extends State<_SlideWebView> {
  late final WebViewController _controller;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..loadHtmlString(_buildHtml(widget.base64));
  }

  @override
  Widget build(BuildContext context) {
    return WebViewWidget(controller: _controller);
  }

  static const String _pptxCdn =
      'https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master';

  static String _buildHtml(String base64) {
    return '''
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="$_pptxCdn/css/pptxjs.css">
<style>
  body { margin: 0; background: #e8e8e8; padding: 8px; }
  #result .slide { margin: 10px auto; box-shadow: 0 1px 4px rgba(0,0,0,.25); }
</style>
</head>
<body>
<div id="result"></div>
<script src="$_pptxCdn/js/jquery-1.11.3.min.js"></script>
<script src="$_pptxCdn/js/jszip.min.js"></script>
<script src="$_pptxCdn/js/filereader.js"></script>
<script src="$_pptxCdn/js/dingbat.js"></script>
<script src="$_pptxCdn/js/pptxjs.js"></script>
<script src="$_pptxCdn/js/divs2slides.js"></script>
<script>
  var b64 = '$base64';
  var bin = atob(b64);
  var bytes = new Uint8Array(bin.length);
  for (var i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  var blob = new Blob([bytes],
      {type: 'application/vnd.openxmlformats-officedocument.presentationml.presentation'});
  var url = URL.createObjectURL(blob);
  \$("#result").pptxToHtml({pptxFileUrl: url, slideMode: false, mediaProcess: true});
</script>
</body>
</html>
''';
  }
}
