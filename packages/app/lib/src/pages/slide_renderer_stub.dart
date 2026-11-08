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
<link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/css/pptxjs.css" onerror="window.__pptxCssError=true">
<style>
body{margin:0;background:#e8e8e8;padding:8px;font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif}
#result .slide{margin:10px auto;box-shadow:0 1px 4px rgba(0,0,0,.25)}
#pptx-status{padding:24px;color:#555;text-align:center;line-height:1.6}
#pptx-status.error{color:#b42318}
</style>
</head><body>
<div id="pptx-status">正在加载课件…</div>
<div id="result"></div>
<script>
function pptxStatus(msg,isErr){var el=document.getElementById('pptx-status'); if(!el)return; el.textContent=msg; el.className=isErr?'error':''; el.style.display=msg?'block':'none';}
window.addEventListener('error',function(e){ if(e && e.target && (e.target.src||e.target.href)){ pptxStatus('课件渲染组件加载失败（网络可能无法访问外部资源）。请重试或更换网络。',true); } },true);
setTimeout(function(){ if(!window.jQuery||!jQuery.fn||!jQuery.fn.pptxToHtml){ pptxStatus('课件渲染组件未就绪，请检查网络后重试。',true); } },15000);
</script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/jquery-1.11.3.min.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/jszip.min.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/filereader.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/dingbat.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/pptxjs.js"></script>
<script src="https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/js/divs2slides.js"></script>
<script>
try{
  var b64='$base64',bin=atob(b64),bytes=new Uint8Array(bin.length);
  for(var i=0;i<bin.length;i++)bytes[i]=bin.charCodeAt(i);
  var blob=new Blob([bytes],{type:'application/vnd.openxmlformats-officedocument.presentationml.presentation'});
  var url=URL.createObjectURL(blob);
  if(!window.jQuery||!jQuery.fn||!jQuery.fn.pptxToHtml){ throw new Error('renderer missing'); }
  jQuery("#result").pptxToHtml({pptxFileUrl:url,slideMode:false,mediaProcess:true});
  setTimeout(function(){ var r=document.getElementById('result'); if(r && r.children && r.children.length>0){ pptxStatus('',false); } else { pptxStatus('课件内容解析未完成，请重试。若多次失败，可能是课件格式较复杂或网络受限。',true); } },6000);
}catch(e){ pptxStatus('课件解析失败：'+(e&&e.message?e.message:e),true); }
</script>
</body></html>''';
