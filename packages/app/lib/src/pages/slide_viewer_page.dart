import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'slide_renderer_stub.dart'
    if (dart.library.html) 'slide_renderer_web.dart';

/// 课件（PPTX）阅读页：拉取二进制为 base64，交给 pptxjs 在 WebView 内渲染。
///
/// 端内渲染（不可下载）满足防盗要求；pptxjs 由 jsDelivr CDN 加载。
/// 签名过期时会重取资源清单并用最新签名重试一次；CDN 不可达会导致渲染容器空白，
/// 此处仅覆盖“资源拉取”失败，渲染失败需看浏览器/网络到 jsDelivr 的连通性。
class SlideViewerPage extends StatefulWidget {
  const SlideViewerPage({
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
  State<SlideViewerPage> createState() => _SlideViewerPageState();
}

class _SlideViewerPageState extends State<SlideViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<String> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<String> _load() async {
    var current = widget.resource;
    return _api.fetchResourceBase64(
      current,
      refresher: (widget.courseCode != null && widget.lectureNo != null)
          ? () => _api.refreshLectureResource(
              widget.courseCode!, widget.lectureNo!, current.storageKey)
          : null,
    );
  }

  String _errorMessage(Object? error) {
    if (error is DioException) {
      final code = error.response?.statusCode ?? 0;
      if (code == 401) return '登录状态已失效，请重新登录后再试';
      if (code == 403) return '课件链接已失效或没有访问权限，请重试';
      if (code == 404) return '课件不存在或已下架';
      if (error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.receiveTimeout ||
          error.type == DioExceptionType.sendTimeout) {
        return '网络超时，请检查网络后重试';
      }
      if (error.type == DioExceptionType.connectionError) {
        return '网络连接失败，请检查网络后重试';
      }
    }
    return '课件加载失败，请重试';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.resource.title)),
      body: FutureBuilder<String>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<String> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError || (snapshot.data ?? '').isEmpty) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Text(_errorMessage(snapshot.error),
                        textAlign: TextAlign.center),
                    const SizedBox(height: 8),
                    const Text(
                      '若课件多次失败，可能是网络限制了渲染组件加载；可稍后重试或更换网络。',
                      textAlign: TextAlign.center,
                      style: TextStyle(fontSize: 12, color: Colors.grey),
                    ),
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
          return SlideRenderer(base64: snapshot.data!);
        },
      ),
    );
  }
}
