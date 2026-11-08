import 'dart:async';

import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

/// 视频播放页：播放资源清单下发的 COS 预签名直链（自认证 URL，无需带会话头）。
///
/// 默认**有声播放**。浏览器若拦截带声音的自动播放，会停在首帧并提示用户点击播放；
/// 用户点击后以音量 1 播放（不再强制静音）。控件条浮动显示，播放中 3 秒后自动隐藏，
/// 点击画面可再次唤出。
///
/// [urlLoader] 在初始化/播放失败时可重取最新媒体地址（签名过期或直链失效后重试）。
class VideoPlayerPage extends StatefulWidget {
  const VideoPlayerPage({
    super.key,
    required this.title,
    required this.url,
    this.urlLoader,
  });

  final String title;
  final String url;
  final Future<String> Function()? urlLoader;

  @override
  State<VideoPlayerPage> createState() => _VideoPlayerPageState();
}

class _VideoPlayerPageState extends State<VideoPlayerPage> {
  VideoPlayerController? _controller;
  bool _ready = false;
  bool _failed = false;
  bool _retrying = false;
  bool _muted = false;
  bool _controlsVisible = true;
  double _speed = 1.0;
  String _errorDetail = '';
  String _currentUrl = '';
  String? _playHint;
  Timer? _hideTimer;

  static const List<double> _speedOptions = <double>[
    0.5,
    0.75,
    1.0,
    1.25,
    1.5,
    2.0,
  ];

  @override
  void initState() {
    super.initState();
    _currentUrl = widget.url;
    unawaited(_initController(_currentUrl));
  }

  Future<void> _initController(String url, {bool forceRefresh = false}) async {
    if (!mounted) return;
    setState(() {
      _ready = false;
      _failed = false;
      _errorDetail = '';
      _playHint = null;
      _retrying = forceRefresh;
    });
    var target = url;
    if (forceRefresh && widget.urlLoader != null) {
      try {
        target = await widget.urlLoader!();
        _currentUrl = target;
      } catch (e) {
        if (!mounted) return;
        setState(() {
          _failed = true;
          _retrying = false;
          _errorDetail = e.toString();
        });
        return;
      }
    }
    final old = _controller;
    old?.removeListener(_onControllerChanged);
    final controller = VideoPlayerController.networkUrl(Uri.parse(target));
    _controller = controller;
    controller.addListener(_onControllerChanged);
    try {
      await controller.initialize();
      if (!mounted || _controller != controller) {
        await controller.dispose();
        return;
      }
      await old?.dispose();
      setState(() {
        _ready = true;
        _failed = false;
        _retrying = false;
      });
      await _startPlayback();
    } catch (e) {
      if (!mounted || _controller != controller) {
        await controller.dispose();
        return;
      }
      await old?.dispose();
      if (!forceRefresh && widget.urlLoader != null) {
        await _initController(target, forceRefresh: true);
        return;
      }
      setState(() {
        _failed = true;
        _retrying = false;
        _errorDetail = e.toString();
      });
    }
  }

  void _onControllerChanged() {
    if (!mounted) return;
    setState(() {});
  }

  VideoPlayerController? get _c => _controller;

  Future<void> _startPlayback() async {
    final controller = _c;
    if (controller == null) return;
    try {
      await controller.setVolume(_muted ? 0 : 1);
      await controller.play();
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
    final controller = _c;
    if (controller == null) return;
    setState(() => _playHint = null);
    try {
      await controller.setVolume(_muted ? 0 : 1);
      await controller.play();
      _showControls();
    } catch (e) {
      if (!mounted) return;
      if (widget.urlLoader != null) {
        await _initController(_currentUrl, forceRefresh: true);
        return;
      }
      setState(() => _errorDetail = e.toString());
    }
  }

  void _togglePlay() {
    final controller = _c;
    if (controller == null) return;
    if (controller.value.isPlaying) {
      controller.pause();
      _showControls();
    } else {
      unawaited(_playFromUser());
    }
  }

  void _showControls() {
    setState(() => _controlsVisible = true);
    _hideTimer?.cancel();
    _hideTimer = Timer(const Duration(seconds: 3), () {
      final controller = _c;
      if (!mounted || controller == null || !controller.value.isPlaying) return;
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
    final controller = _c;
    if (controller == null) return;
    final next = !_muted;
    await controller.setVolume(next ? 0 : 1);
    if (!mounted) return;
    setState(() => _muted = next);
    // 解除静音后若已暂停，直接继续有声播放。
    if (!next && !controller.value.isPlaying) {
      await controller.play();
    }
    _showControls();
  }

  Future<void> _setSpeed(double speed) async {
    final controller = _c;
    if (controller == null) return;
    await controller.setPlaybackSpeed(speed);
    if (!mounted) return;
    setState(() => _speed = speed);
    _showControls();
  }

  @override
  void dispose() {
    _hideTimer?.cancel();
    _controller?.removeListener(_onControllerChanged);
    unawaited(_controller?.dispose());
    super.dispose();
  }

  String _fmt(Duration d) {
    final m = d.inMinutes;
    final s = d.inSeconds % 60;
    return '$m:${s.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final controller = _controller;
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
                        maxLines: 6,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                    const SizedBox(height: 16),
                    FilledButton(
                      onPressed: _retrying
                          ? null
                          : () => unawaited(
                              _initController(_currentUrl, forceRefresh: true)),
                      child: Text(_retrying ? '重试中…' : '重试'),
                    ),
                  ],
                ),
              ),
            )
          : (!_ready || controller == null)
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      const CircularProgressIndicator(),
                      if (_retrying) ...<Widget>[
                        const SizedBox(height: 12),
                        const Text('正在刷新播放地址…',
                            style: TextStyle(color: Colors.white70)),
                      ],
                    ],
                  ),
                )
              : Column(
                  children: <Widget>[
                    Expanded(
                      child: Center(
                        child: AspectRatio(
                          // 部分流式/Web 源在首帧前 aspectRatio 为 0，回退 16:9。
                          aspectRatio: controller.value.aspectRatio > 0
                              ? controller.value.aspectRatio
                              : 16 / 9,
                          child: Stack(
                            fit: StackFit.expand,
                            children: <Widget>[
                              VideoPlayer(controller),
                              // 点击画面：显示/隐藏浮动控件。
                              GestureDetector(
                                behavior: HitTestBehavior.opaque,
                                onTap: _toggleControls,
                              ),
                              if (!controller.value.isPlaying)
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
                                    child: _controlsBar(context, controller),
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

  Widget _controlsBar(BuildContext context, VideoPlayerController controller) {
    final value = controller.value;
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
            controller,
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
                  size: 24,
                ),
                label: Text(
                  _muted ? '静音' : '音量',
                  style: const TextStyle(color: Colors.white),
                ),
              ),
              PopupMenuButton<double>(
                initialValue: _speed,
                tooltip: '播放速度',
                onSelected: _setSpeed,
                itemBuilder: (BuildContext context) => <PopupMenuEntry<double>>[
                  for (final double option in _speedOptions)
                    PopupMenuItem<double>(
                      value: option,
                      child: Text('${option}x'),
                    ),
                ],
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 10),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      const Icon(Icons.speed, color: Colors.white, size: 22),
                      const SizedBox(width: 4),
                      Text(
                        '${_speed}x',
                        style: const TextStyle(color: Colors.white),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
