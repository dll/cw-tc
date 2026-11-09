import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'html_media_unlock.dart';
import 'html_viewer_stub.dart' if (dart.library.html) 'html_viewer_web.dart';

/// 资源阅读页：文本类资源端内展示。
///
/// - markdown → 渲染 Markdown（讲义）
/// - html → iframe（Web）/ WebView（Android）渲染
/// - code → 纯文本（可复制）
///
/// 签名资源可能因停留过久失效；失败时会重取同一讲资源清单并用最新签名重试一次，
/// 避免“页面已打开但重试仍复用旧签名”导致持续失败。
class ResourceViewerPage extends StatefulWidget {
  const ResourceViewerPage({
    super.key,
    required this.resource,
    this.api,
    this.courseCode,
    this.lectureNo,
  });

  final LectureResource resource;
  final ApiClient? api;
  final String? courseCode;
  final int? lectureNo;

  @override
  State<ResourceViewerPage> createState() => _ResourceViewerPageState();
}

class _ResourceViewerPageState extends State<ResourceViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<_LoadedText> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<_LoadedText> _load() async {
    var current = widget.resource;
    // 二进制预览资源（图片/PDF）不按 UTF-8 文本加载，避免乱码/无意义重试。
    if (_isBinaryPreview(current)) {
      if (_canRefresh) {
        final fresh = await _refresh(current);
        if (fresh != null) current = fresh;
      }
      return _LoadedText(current, '');
    }
    final text = await _api.fetchResourceText(
      current,
      refresher: _canRefresh ? () => _refresh(current) : null,
    );
    // refresher 可能返回新条目；为保持 HTML 内联基址等元数据一致，成功后再刷新一次元数据。
    if (_canRefresh) {
      final fresh = await _refresh(current);
      if (fresh != null) current = fresh;
    }
    return _LoadedText(current, text);
  }

  bool _isBinaryPreview(LectureResource r) => r.isImageKind || r.isPdfKind;

  bool get _canRefresh => widget.courseCode != null && widget.lectureNo != null;

  Future<LectureResource?> _refresh(LectureResource current) =>
      _api.refreshLectureResource(
          widget.courseCode!, widget.lectureNo!, current.storageKey);

  String _errorMessage(Object error) {
    if (error is DioException) {
      final code = error.response?.statusCode ?? 0;
      if (code == 401) return '登录状态已失效，请重新登录后再试';
      if (code == 403) return '没有访问权限，或资源链接已失效，请重试';
      if (code == 404) return '资源不存在或已下架';
      if (error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.receiveTimeout ||
          error.type == DioExceptionType.sendTimeout) {
        return '网络超时，请检查网络后重试';
      }
      if (error.type == DioExceptionType.connectionError) {
        return '网络连接失败，请检查网络后重试';
      }
    }
    return '资源加载失败，请重试';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.resource.fileName)),
      body: FutureBuilder<_LoadedText>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<_LoadedText> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Text(_errorMessage(snapshot.error!),
                        textAlign: TextAlign.center),
                    const SizedBox(height: 12),
                    FilledButton(
                      onPressed: () => setState(() => _future = _load()),
                      child: const Text('重试'),
                    ),
                  ],
                ),
              ),
            );
          }
          final loaded = snapshot.data;
          if (loaded == null) {
            return const Center(child: Text('资源加载失败，请重试'));
          }
          final content = loaded.text;
          final resource = loaded.resource;
          if (resource.isImageKind) {
            return _SignedImage(
              resource: resource,
              api: _api,
              courseCode: widget.courseCode,
              lectureNo: widget.lectureNo,
            );
          }
          if (resource.isPdfKind) {
            return _UnsupportedInline(
              message: '当前版本暂不支持端内直接预览 PDF，请联系管理员或在支持的环境中查看。',
              detail: resource.fileName,
            );
          }
          switch (resource.kind) {
            case 'markdown':
              return Markdown(data: content, selectable: true);
            case 'html':
              final html = resource.inlineBaseUrl == null
                  ? content
                  : htmlWithBase(
                      content,
                      _api.mediaUrl(resource.inlineBaseUrl!),
                    );
              return HtmlViewer(html: htmlWithMediaUnlock(html));
            case 'code':
            case 'frozen_output':
            default:
              if (content.trim().isEmpty && !_isBinaryPreview(resource)) {
                return _UnsupportedInline(
                  message: '该资源内容为空或暂不支持端内展示。',
                  detail: '${resource.kind} · ${resource.fileName}',
                );
              }
              return SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  content.isEmpty ? '（无可显示文本内容）' : content,
                  style: const TextStyle(
                      fontFamily: 'monospace', fontSize: 14, height: 1.5),
                ),
              );
          }
        },
      ),
    );
  }
}

class _LoadedText {
  const _LoadedText(this.resource, this.text);
  final LectureResource resource;
  final String text;
}

class _SignedImage extends StatefulWidget {
  const _SignedImage({
    required this.resource,
    required this.api,
    this.courseCode,
    this.lectureNo,
  });

  final LectureResource resource;
  final ApiClient api;
  final String? courseCode;
  final int? lectureNo;

  @override
  State<_SignedImage> createState() => _SignedImageState();
}

class _SignedImageState extends State<_SignedImage> {
  late Future<List<int>> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<List<int>> _load() {
    final canRefresh = widget.courseCode != null && widget.lectureNo != null;
    var current = widget.resource;
    return widget.api.fetchResourceBytes(
      current,
      refresher: canRefresh
          ? () => widget.api.refreshLectureResource(
              widget.courseCode!, widget.lectureNo!, current.storageKey)
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final lower = widget.resource.fileName.toLowerCase();
    if (lower.endsWith('.svg')) {
      return const _UnsupportedInline(
        message: '当前版本暂不支持端内直接预览 SVG 图片。',
      );
    }
    return FutureBuilder<List<int>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }
        if (snapshot.hasError || (snapshot.data?.isEmpty ?? true)) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  Text('图片加载失败，请重试。\n${widget.resource.fileName}',
                      textAlign: TextAlign.center),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: () => setState(() => _future = _load()),
                    child: const Text('重试'),
                  ),
                ],
              ),
            ),
          );
        }
        return InteractiveViewer(
          minScale: 0.5,
          maxScale: 5,
          child: Center(
            child: Image.memory(
              Uint8List.fromList(snapshot.data!),
              fit: BoxFit.contain,
              gaplessPlayback: true,
              errorBuilder: (context, error, stack) => Center(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Text('图片无法显示，请返回重试。\n${widget.resource.fileName}',
                      textAlign: TextAlign.center),
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}


class _PdfPreviewFallback extends StatefulWidget {
  const _PdfPreviewFallback({
    required this.resource,
    required this.api,
    this.courseCode,
    this.lectureNo,
  });

  final LectureResource resource;
  final ApiClient api;
  final String? courseCode;
  final int? lectureNo;

  @override
  State<_PdfPreviewFallback> createState() => _PdfPreviewFallbackState();
}

class _PdfPreviewFallbackState extends State<_PdfPreviewFallback> {
  late Future<List<int>> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<List<int>> _load() {
    final canRefresh = widget.courseCode != null && widget.lectureNo != null;
    var current = widget.resource;
    return widget.api.fetchResourceBytes(
      current,
      refresher: canRefresh
          ? () => widget.api.refreshLectureResource(
              widget.courseCode!, widget.lectureNo!, current.storageKey)
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<List<int>>(
      future: _future,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: CircularProgressIndicator());
        }
        if (snapshot.hasError || (snapshot.data?.isEmpty ?? true)) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Icon(Icons.picture_as_pdf_outlined, size: 40),
                  const SizedBox(height: 12),
                  Text('PDF 加载失败，请重试。\n${widget.resource.fileName}',
                      textAlign: TextAlign.center),
                  const SizedBox(height: 12),
                  FilledButton(
                    onPressed: () => setState(() => _future = _load()),
                    child: const Text('重试'),
                  ),
                ],
              ),
            ),
          );
        }
        return Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                const Icon(Icons.picture_as_pdf_outlined, size: 40),
                const SizedBox(height: 12),
                Text(widget.resource.fileName, textAlign: TextAlign.center),
                const SizedBox(height: 8),
                Text(
                  'PDF 文件可正常获取（${(snapshot.data!.length / 1024).toStringAsFixed(0)} KB），当前 Flutter Web 端暂未集成 PDF 内嵌预览组件；请在移动端/桌面端或后续版本查看。',
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 13, color: Colors.black54),
                ),
                const SizedBox(height: 12),
                FilledButton(
                  onPressed: () => setState(() => _future = _load()),
                  child: const Text('重新检测'),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

class _UnsupportedInline extends StatelessWidget {
  const _UnsupportedInline({required this.message, this.detail});

  final String message;
  final String? detail;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            const Icon(Icons.insert_drive_file_outlined, size: 40),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center),
            if (detail != null) ...<Widget>[
              const SizedBox(height: 8),
              Text(detail!,
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 12, color: Colors.grey)),
            ],
          ],
        ),
      ),
    );
  }
}

/// 把受控的同讲资源地址放在原页面第一个 base 位置，供相对音频路径解析。
String htmlWithBase(String html, String baseUrl) {
  final base = '<base href="$baseUrl">';
  final head = RegExp(r'<head(?:\s[^>]*)?>', caseSensitive: false);
  final match = head.firstMatch(html);
  if (match == null) return '$base$html';
  return html.replaceRange(match.end, match.end, base);
}
