import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

/// 视频播放页：播放资源清单下发的 COS 预签名直链（自认证 URL，无需带会话头）。
///
/// Web 与 App 的差异：浏览器**禁止带声音的自动播放**，故 Web 端默认**静音起播**，
/// 用户点「点此出声」再解除静音；App（Android）直接带声播放。
class VideoPlayerPage extends StatefulWidget {
  const VideoPlayerPage({super.key, required this.title, required this.url});

  final String title;
  final String url;

  @override
  State<VideoPlayerPage> createState() => _VideoPlayerPageState();
}

class _VideoPlayerPageState extends State<VideoPlayerPage> {
  late final VideoPlayerController _controller;
  bool _ready = false;
  bool _failed = false;

  /// Web 默认静音：静音自动播放不受浏览器策略拦截；用户点按钮再出声。
  late bool _muted = kIsWeb;
  String _errorDetail = '';
  String? _playHint;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(Uri.parse(widget.url));
    _controller.initialize().then((_) async {
      if (!mounted) return;
      setState(() => _ready = true);
      _controller.setVolume(_muted ? 0 : 1);
      try {
        await _controller.play();
      } catch (_) {
        // 浏览器拦截了自动播放：停在第一帧，提示用户手动播放。
        if (!mounted) return;
        setState(() => _playHint = '浏览器已拦截自动播放，请点播放键开始');
      }
    }).catchError((Object e) {
      if (!mounted) return;
      setState(() {
        _failed = true;
        _errorDetail = e.toString();
      });
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _togglePlay() {
    setState(() {
      if (_controller.value.isPlaying) {
        _controller.pause();
      } else {
        _playHint = null;
        _controller.play();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.title)),
      body: _failed
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    const Icon(Icons.error_outline, size: 48),
                    const SizedBox(height: 12),
                    const Text('视频加载失败，请稍后重试'),
                    if (_errorDetail.isNotEmpty) ...<Widget>[
                      const SizedBox(height: 8),
                      Text(
                        _errorDetail,
                        style: const TextStyle(fontSize: 12, color: Colors.grey),
                        textAlign: TextAlign.center,
                      ),
                    ],
                  ],
                ),
              ),
            )
          : !_ready
              ? const Center(child: CircularProgressIndicator())
              : Column(
                  children: <Widget>[
                    AspectRatio(
                      // 部分流式/Web 源在首帧前 aspectRatio 为 0，回退 16:9 避免布局报错。
                      aspectRatio: _controller.value.aspectRatio > 0
                          ? _controller.value.aspectRatio
                          : 16 / 9,
                      child: VideoPlayer(_controller),
                    ),
                    VideoProgressIndicator(
                      _controller,
                      allowScrubbing: true,
                      padding: const EdgeInsets.symmetric(vertical: 8),
                    ),
                    if (_playHint != null)
                      Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 16),
                        child: Text(
                          _playHint!,
                          style: const TextStyle(fontSize: 12, color: Colors.grey),
                          textAlign: TextAlign.center,
                        ),
                      ),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: <Widget>[
                        IconButton(
                          icon: Icon(
                            _controller.value.isPlaying
                                ? Icons.pause_circle_filled
                                : Icons.play_circle_filled,
                            size: 48,
                          ),
                          onPressed: _togglePlay,
                        ),
                        // 音量：图标 + 文字，明显可点（静音自动播放后需要出声入口）。
                        TextButton.icon(
                          onPressed: () => setState(() {
                            _muted = !_muted;
                            _controller.setVolume(_muted ? 0 : 1);
                          }),
                          icon: Icon(
                            _muted ? Icons.volume_off : Icons.volume_up,
                            size: 28,
                          ),
                          label: Text(_muted ? '已静音 · 点此出声' : '静音'),
                        ),
                      ],
                    ),
                    const SizedBox(height: 16),
                  ],
                ),
    );
  }
}
