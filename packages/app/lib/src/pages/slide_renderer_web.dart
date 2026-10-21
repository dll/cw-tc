// ignore_for_file: avoid_web_libraries_in_flutter
import 'dart:html' as dhtml;
import 'dart:ui_web' as ui_web;

import 'package:flutter/widgets.dart';

/// Web 使用 iframe 渲染 PPTX.js，避免调用不支持 Web 的 webview_flutter。
class SlideRenderer extends StatelessWidget {
  const SlideRenderer({super.key, required this.base64});

  final String base64;

  @override
  Widget build(BuildContext context) {
    final viewType = 'pptx-iframe-${identityHashCode(this)}';
    ui_web.platformViewRegistry.registerViewFactory(viewType, (int viewId) {
      return dhtml.IFrameElement()
        ..srcdoc = buildPptxHtml(base64)
        ..style.width = '100%'
        ..style.height = '100%'
        ..style.border = 'none';
    });
    return HtmlElementView(viewType: viewType);
  }
}

String buildPptxHtml(String base64) => '''
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
