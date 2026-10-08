import 'package:audioplayers/audioplayers.dart';
import 'package:flutter/material.dart';

/// 音频播放页：播放资源清单下发的 COS 预签名直链（分段音频）。
class AudioPlayerPage extends StatefulWidget {
  const AudioPlayerPage({super.key, required this.title, required this.url});

  final String title;
  final String url;

  @override
  State<AudioPlayerPage> createState() => _AudioPlayerPageState();
}

class _AudioPlayerPageState extends State<AudioPlayerPage> {
  final AudioPlayer _player = AudioPlayer();
  bool _playing = false;
  bool _failed = false;
  Duration _position = Duration.zero;
  Duration _duration = Duration.zero;

  @override
  void initState() {
    super.initState();
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
    _play();
  }

  Future<void> _play() async {
    try {
      await _player.play(UrlSource(widget.url));
    } catch (_) {
      if (!mounted) return;
      setState(() => _failed = true);
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
          ? const Center(child: Text('音频加载失败，请稍后重试'))
          : Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    const Icon(Icons.audiotrack, size: 64),
                    const SizedBox(height: 16),
                    Text(widget.title, textAlign: TextAlign.center),
                    const SizedBox(height: 16),
                    Slider(
                      value: valueMs.toDouble(),
                      max: (maxMs < 1 ? 1 : maxMs).toDouble(),
                      onChanged: (double v) =>
                          _player.seek(Duration(milliseconds: v.toInt())),
                    ),
                    Text('${_fmt(_position)} / ${_fmt(_duration)}'),
                    const SizedBox(height: 8),
                    IconButton(
                      iconSize: 56,
                      icon: Icon(_playing
                          ? Icons.pause_circle_filled
                          : Icons.play_circle_filled),
                      onPressed: () {
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
