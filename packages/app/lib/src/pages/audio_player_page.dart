import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';

/// 音频播放页：播放资源清单下发的 COS 预签名直链（分段音频）。
///
/// [urlLoader] 在加载/播放失败时可重取最新媒体地址（签名过期或直链失效后重试）。
class AudioPlayerPage extends StatefulWidget {
  const AudioPlayerPage({
    super.key,
    required this.title,
    required this.url,
    this.urlLoader,
  });

  final String title;
  final String url;
  final Future<String> Function()? urlLoader;

  @override
  State<AudioPlayerPage> createState() => _AudioPlayerPageState();
}

class _AudioPlayerPageState extends State<AudioPlayerPage> {
  final AudioPlayer _player = AudioPlayer();
  bool _playing = false;
  bool _failed = false;
  bool _loading = true;
  String _errorDetail = '';
  String _currentUrl = '';
  Duration _position = Duration.zero;
  Duration _duration = Duration.zero;

  @override
  void initState() {
    super.initState();
    _currentUrl = widget.url;
    _player.onPlayerStateChanged.listen((PlayerState s) {
      if (!mounted) return;
      setState(() => _playing = s == PlayerState.playing);
    });
    _player.onPositionChanged.listen((Duration d) {
      if (!mounted) return;
      setState(() => _position = d);
    });
    _player.onDurationChanged.listen((Duration d) {
      if (!mounted) return;
      setState(() => _duration = d);
    });
    _player.onPlayerComplete.listen((_) {
      if (!mounted) return;
      setState(() => _playing = false);
    });
    _play(_currentUrl);
  }

  Future<void> _play(String url, {bool forceRefresh = false}) async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _failed = false;
      _errorDetail = '';
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
          _loading = false;
          _errorDetail = e.toString();
        });
        return;
      }
    }
    try {
      await _player.stop();
      await _player.play(UrlSource(target));
      if (!mounted) return;
      setState(() => _loading = false);
    } catch (e) {
      if (!mounted) return;
      if (!forceRefresh && widget.urlLoader != null) {
        await _play(target, forceRefresh: true);
        return;
      }
      setState(() {
        _failed = true;
        _loading = false;
        _errorDetail = e.toString();
      });
    }
  }

  @override
  void dispose() {
    _player.dispose();
    super.dispose();
  }

  String _fmt(Duration d) {
    final m = d.inMinutes;
    final s = d.inSeconds % 60;
    return '$m:${s.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    final maxMs = _duration.inMilliseconds;
    final valueMs = _position.inMilliseconds.clamp(0, maxMs < 1 ? 1 : maxMs);
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
                    const Text('音频加载失败，请稍后重试'),
                    if (_errorDetail.isNotEmpty) ...<Widget>[
                      const SizedBox(height: 8),
                      Text(_errorDetail,
                          style:
                              const TextStyle(fontSize: 12, color: Colors.grey),
                          textAlign: TextAlign.center,
                          maxLines: 5,
                          overflow: TextOverflow.ellipsis),
                    ],
                    const SizedBox(height: 16),
                    FilledButton(
                      onPressed: () => _play(_currentUrl, forceRefresh: true),
                      child: const Text('重试'),
                    ),
                  ],
                ),
              ),
            )
          : Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    if (_loading) const LinearProgressIndicator(),
                    const SizedBox(height: 12),
                    const Icon(Icons.audiotrack, size: 64),
                    const SizedBox(height: 16),
                    Text(widget.title, textAlign: TextAlign.center),
                    const SizedBox(height: 16),
                    Slider(
                      value: valueMs.toDouble(),
                      max: (maxMs < 1 ? 1 : maxMs).toDouble(),
                      onChanged: _loading
                          ? null
                          : (double v) =>
                              _player.seek(Duration(milliseconds: v.toInt())),
                    ),
                    Text('${_fmt(_position)} / ${_fmt(_duration)}'),
                    const SizedBox(height: 8),
                    IconButton(
                      iconSize: 56,
                      icon: Icon(_playing
                          ? Icons.pause_circle_filled
                          : Icons.play_circle_filled),
                      onPressed: _loading
                          ? null
                          : () {
                              _playing ? _player.pause() : _player.resume();
                            },
                    ),
                  ],
                ),
              ),
            ),
    );
  }
}
