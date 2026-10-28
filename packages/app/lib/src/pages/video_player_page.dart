import 'dart:async';

import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

/// 视频播放页：播放资源清单下发的 COS 预签名直链（自认证 URL，无需带会话头）。
///
/// 默认**有声播放**。浏览器若拦截带声音的自动播放，会停在首帧并提示用户点击播放；
/// 用户点击后以音量 1 播放（不再强制静音）。控件条浮动显示，播放中 3 秒后自动隐藏，
/// 点击画面可再次唤出。
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
  bool _muted = false;
  bool _controlsVisible = true;
  String _errorDetail = '';
  String? _playHint;
  Timer? _hideTimer;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(Uri.parse(widget.url));
    _controller.addListener(_onControllerChanged);
    _controller.initialize().then((_) async {
      if (!mounted) return;
      setState(() => _ready = true);
      await _startPlayback();
    }).catchError((Object e) {
      if (!mounted) return;
      setState(() {
        _failed = true;
        _errorDetail = e.toString();
      });
    });
  }

  void _onControllerChanged() {
    if (!mounted) return;
    setState(() {});
  }

  Future<void> _startPlayback() async {
    try {
      await _controller.setVolume(_muted ? 0 : 1);
      await _controller.play();
      if (!mounted) return;
      setState(() => _playHint = null);
      _showControls();
    } catch (_) {
      if (!mounted) return;
      setState(() {
        // 浏览器拦截带声音的自动播放：保留声音设置，等待用户点击。
        _playHint = '浏览器已拦截自动播放，点击播放即可有声播放';
        _controlsVisible = true;
      });
      _hideTimer?.cancel();
    }
  }

  Future<void> _playFromUser() async {
    setState(() => _playHint = null);
    await _controller.setVolume(_muted ? 0 : 1);
    await _controller.play();
    _showControls();
  }

  void _togglePlay() {
    if (_controller.value.isPlaying) {
      _controller.pause();
      _showControls();
    } else {
      _playFromUser();
    }
  }

  void _showControls() {
    setState(() => _controlsVisible = true);
    _hideTimer?.cancel();
    _hideTimer = Timer(const Duration(seconds: 3), () {
      if (!mounted || !_controller.value.isPlaying) return;
      setState(() => _controlsVisible = false);
    });
  }

  void _toggleControls() {
    if (_controlsVisible) {
      _hideTimer?.cancel();
      setState(() => _controlsVisible = false);
    } else {
      _showControls();
    }
  }

  Future<void> _toggleMute() async {
    final next = !_muted;
    await _controller.setVolume(next ? 0 : 1);
    if (!mounted) return;
    setState(() => _muted = next);
    // 解除静音后若已暂停，直接继续有声播放。
    if (!next && !_controller.value.isPlaying) {
      await _controller.play();
    }
    _showControls();
  }

  @override
  void dispose() {
    _hideTimer?.cancel();
    _controller.removeListener(_onControllerChanged);
    _controller.dispose();
    super.dispose();
  }

  String _fmt(Duration d) {
    final m = d.inMinutes;
    final s = d.inSeconds % 60;
    return '$m:${s.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.title)),
      backgroundColor: Colors.black,
      body: _failed
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    const Icon(Icons.error_outline, size: 48, color: Colors.white70),
                    const SizedBox(height: 12),
                    const Text('视频加载失败，请稍后重试',
                        style: TextStyle(color: Colors.white70)),
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
                    Expanded(
                      child: Center(
                        child: AspectRatio(
                          // 部分流式/Web 源在首帧前 aspectRatio 为 0，回退 16:9。
                          aspectRatio: _controller.value.aspectRatio > 0
                              ? _controller.value.aspectRatio
                              : 16 / 9,
                          child: Stack(
                            fit: StackFit.expand,
                            children: <Widget>[
                              VideoPlayer(_controller),
                              // 点击画面：显示/隐藏浮动控件。
                              GestureDetector(
                                behavior: HitTestBehavior.opaque,
                                onTap: _toggleControls,
                              ),
                              if (!_controller.value.isPlaying)
                                Center(
                                  child: IconButton(
                                    iconSize: 72,
                                    color: Colors.white,
                                    icon: const Icon(Icons.play_circle_fill),
                                    onPressed: _playFromUser,
                                  ),
                                ),
                              AnimatedOpacity(
                                opacity: _controlsVisible ? 1 : 0,
                                duration: const Duration(milliseconds: 200),
                                child: IgnorePointer(
                                  ignoring: !_controlsVisible,
                                  child: Align(
                                    alignment: Alignment.bottomCenter,
                                    child: _controlsBar(context),
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                    if (_playHint != null)
                      Padding(
                        padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                        child: Text(
                          _playHint!,
                          style: const TextStyle(fontSize: 12, color: Colors.grey),
                          textAlign: TextAlign.center,
                        ),
                      ),
                  ],
                ),
    );
  }

  Widget _controlsBar(BuildContext context) {
    final value = _controller.value;
    return Container(
      padding: const EdgeInsets.fromLTRB(8, 32, 8, 8),
      decoration: const BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: <Color>[Colors.transparent, Colors.black87],
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          VideoProgressIndicator(
            _controller,
            allowScrubbing: true,
            padding: const EdgeInsets.symmetric(vertical: 4),
          ),
          Row(
            children: <Widget>[
              IconButton(
                color: Colors.white,
                icon: Icon(value.isPlaying
                    ? Icons.pause_circle_filled
                    : Icons.play_circle_filled),
                onPressed: _togglePlay,
              ),
              Text(
                '${_fmt(value.position)} / ${_fmt(value.duration)}',
                style: const TextStyle(color: Colors.white70, fontSize: 12),
              ),
              const Spacer(),
              TextButton.icon(
                onPressed: _toggleMute,
                icon: Icon(
                  _muted ? Icons.volume_off : Icons.volume_up,
                  color: Colors.white,
                  size: 26,
                ),
                label: Text(
                  _muted ? '已静音 · 点此出声' : '有声',
                  style: const TextStyle(color: Colors.white),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
