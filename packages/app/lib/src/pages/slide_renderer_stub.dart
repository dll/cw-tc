import 'package:flutter/widgets.dart';
import 'package:webview_flutter/webview_flutter.dart';

/// Android/iOS 使用系统 WebView 渲染 PPTX.js 页面。
class SlideRenderer extends StatefulWidget {
  const SlideRenderer({super.key, required this.base64});

  final String base64;

  @override
  State<SlideRenderer> createState() => _SlideRendererState();
}

class _SlideRendererState extends State<SlideRenderer> {
  late final WebViewController _controller;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..loadHtmlString(buildPptxHtml(widget.base64));
  }

  @override
  Widget build(BuildContext context) => WebViewWidget(controller: _controller);
}

String buildPptxHtml(String base64) {
  return _pptxHtml(base64);
}

String _pptxHtml(String base64) => '''
<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/css/pptxjs.css">
<style>body{margin:0;background:#e8e8e8;padding:8px}#result .slide{margin:10px auto;box-shadow:0 1px 4px rgba(0,0,0,.25)}</style>
</head><body><div id="result"></div>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/jquery-1.11.3.min.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/jszip.min.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/filereader.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/dingbat.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/pptxjs.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/divs2slides.js"></script>
<script>var b64='$base64',bin=atob(b64),bytes=new Uint8Array(bin.length);
for(var i=0;i<bin.length;i++)bytes[i]=bin.charCodeAt(i);
var blob=new Blob([bytes],{type:'application/vnd.openxmlformats-officedocument.presentationml.presentation'});
var url=URL.createObjectURL(blob);jQuery("#result").pptxToHtml({pptxFileUrl:url,slideMode:false,mediaProcess:true});</script>
</body></html>''';
