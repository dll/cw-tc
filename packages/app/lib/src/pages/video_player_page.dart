import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

/// 视频播放页：播放资源清单下发的 COS 预签名直链（自认证 URL，无需带会话头）。
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
  bool _muted = true;
  String _errorDetail = '';

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(Uri.parse(widget.url));
    _controller.initialize().then((_) {
      if (!mounted) return;
      setState(() => _ready = true);
      // Web 浏览器自动播放策略：未静音的视频不允许无手势自动播放，先静音再播。
      _controller.setVolume(0);
      _controller.play();
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
                      aspectRatio: _controller.value.aspectRatio,
                      child: VideoPlayer(_controller),
                    ),
                    VideoProgressIndicator(
                      _controller,
                      allowScrubbing: true,
                      padding: const EdgeInsets.symmetric(vertical: 8),
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
                          onPressed: () => setState(() {
                            _controller.value.isPlaying
                                ? _controller.pause()
                                : _controller.play();
                          }),
                        ),
                        IconButton(
                          icon: Icon(
                            _muted ? Icons.volume_off : Icons.volume_up,
                            size: 32,
                          ),
                          onPressed: () => setState(() {
                            _muted = !_muted;
                            _controller.setVolume(_muted ? 0 : 1);
                          }),
                        ),
                      ],
                    ),
                    const SizedBox(height: 16),
                  ],
                ),
    );
  }
}
