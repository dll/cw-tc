// ignore_for_file: avoid_web_libraries_in_flutter
import 'dart:html' as dhtml;
import 'dart:ui_web' as ui_web;

import 'package:flutter/widgets.dart';

/// Web 使用 iframe 渲染 PPTX.js，避免调用不支持 Web 的 webview_flutter。
/// 优先加载同源自托管 vendor（/student/vendor/pptxjs/），失败再回退 jsDelivr。
class SlideRenderer extends StatelessWidget {
  const SlideRenderer({super.key, required this.base64});

  final String base64;

  @override
  Widget build(BuildContext context) {
    final viewType = 'pptx-iframe-${identityHashCode(this)}';
    final localBase = Uri.base.resolve('vendor/pptxjs/').toString();
    ui_web.platformViewRegistry.registerViewFactory(viewType, (int viewId) {
      return dhtml.IFrameElement()
        ..srcdoc = buildPptxHtml(base64, localBase: localBase)
        ..style.width = '100%'
        ..style.height = '100%'
        ..style.border = 'none';
    });
    return HtmlElementView(viewType: viewType);
  }
}

String buildPptxHtml(String base64, {String? localBase}) {
  final local = (localBase ?? '').replaceAll("'", r"\'");
  return '''
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
var __pptxLocalBase='$local';
var __pptxCdnBase='https://cdn.jsdelivr.net/gh/meshesha/PPTXjs@master/';
function pptxStatus(msg,isErr){var el=document.getElementById('pptx-status'); if(!el)return; el.textContent=msg; el.className=isErr?'error':''; el.style.display=msg?'block':'none';}
function loadScript(src){return new Promise(function(resolve,reject){var s=document.createElement('script'); s.src=src; s.onload=resolve; s.onerror=function(){reject(new Error(src));}; document.body.appendChild(s);});}
function loadCss(href){return new Promise(function(resolve){var l=document.createElement('link'); l.rel='stylesheet'; l.href=href; l.onload=function(){resolve(true);}; l.onerror=function(){resolve(false);}; document.head.appendChild(l);});}
async function loadFromBase(base){
  await loadCss(base+'css/pptxjs.css');
  var js=['js/jquery-1.11.3.min.js','js/jszip.min.js','js/filereader.js','js/dingbat.js','js/pptxjs.js','js/divs2slides.js'];
  for(var i=0;i<js.length;i++){ await loadScript(base+js[i]); }
}
async function ensureRenderer(){
  if(__pptxLocalBase){ try{ await loadFromBase(__pptxLocalBase); if(window.jQuery&&jQuery.fn&&jQuery.fn.pptxToHtml) return; }catch(e){} }
  await loadFromBase(__pptxCdnBase);
}
(async function(){
  try{
    var b64='$base64',bin=atob(b64),bytes=new Uint8Array(bin.length);
    for(var i=0;i<bin.length;i++)bytes[i]=bin.charCodeAt(i);
    var blob=new Blob([bytes],{type:'application/vnd.openxmlformats-officedocument.presentationml.presentation'});
    var url=URL.createObjectURL(blob);
    await ensureRenderer();
    if(!window.jQuery||!jQuery.fn||!jQuery.fn.pptxToHtml){ throw new Error('课件渲染组件未就绪'); }
    jQuery("#result").pptxToHtml({pptxFileUrl:url,slideMode:false,mediaProcess:true});
    setTimeout(function(){ var r=document.getElementById('result'); if(r && r.children && r.children.length>0){ pptxStatus('',false); } else { pptxStatus('课件内容解析未完成，请重试。若多次失败，可能是课件格式较复杂或浏览器受限。',true); } },7000);
  }catch(e){ pptxStatus('课件加载失败：'+(e&&e.message?e.message:e)+'。请检查网络后重试。',true); }
})();
setTimeout(function(){ if(!window.jQuery||!jQuery.fn||!jQuery.fn.pptxToHtml){ pptxStatus('课件渲染组件加载超时，请检查网络后重试。',true); } },20000);
</script>
</body></html>''';
}
