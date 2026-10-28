/// HTML 课件的媒体增强：强制有声、给视频加原生控件、首次交互后自动续播。
///
/// 浏览器默认可能把 iframe / WebView 里的媒体静音或拦截自动播放；
/// 这里不修改课件源文件，只在渲染前注入一段一次性脚本。
const String _mediaUnlockScript = '''
<script data-cwtc-media-unlock="1">
(function () {
  function unlock(el) {
    if (!el || !el.tagName) return;
    var tag = el.tagName.toLowerCase();
    if (tag !== 'video' && tag !== 'audio') return;
    try { el.muted = false; el.volume = 1; } catch (e) {}
    if (tag === 'video') { el.controls = true; }
  }
  function unlockAll() {
    var nodes = document.querySelectorAll('video,audio');
    for (var i = 0; i < nodes.length; i++) unlock(nodes[i]);
  }
  var originalPlay = HTMLMediaElement.prototype.play;
  HTMLMediaElement.prototype.play = function () {
    unlock(this);
    try { return originalPlay.apply(this, arguments); }
    catch (e) { return Promise.reject(e); }
  };
  document.addEventListener('DOMContentLoaded', unlockAll);
  window.addEventListener('load', unlockAll);
  document.addEventListener('pointerdown', function () {
    unlockAll();
    var nodes = document.querySelectorAll('video,audio');
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      if (el.paused && el.autoplay) { try { el.play(); } catch (e) {} }
    }
  }, true);
  new MutationObserver(unlockAll).observe(document.documentElement, { childList: true, subtree: true });
})();
</script>
''';

/// 把媒体增强脚本插到 </body> 前；已注入则原样返回。
String htmlWithMediaUnlock(String html) {
  if (html.contains('data-cwtc-media-unlock')) return html;
  const script = _mediaUnlockScript;
  final body = RegExp(r'</body\s*>', caseSensitive: false);
  final match = body.firstMatch(html);
  if (match == null) return '$html$script';
  return html.replaceRange(match.start, match.start, script);
}
