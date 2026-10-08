import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../models/course.dart';

/// 后端 API 客户端（M0 骨架）。
///
/// 约定（PRD v2 §9.6）：
///   - 会话凭据**只存系统安全存储**（Keychain / Keystore），绝不写 shared_preferences；
///   - **绝不以 query 传敏感信息** —— 登录与注册一律 POST body；
///     本客户端对 GET 请求做一次断言，把这条铁律变成运行时可见的失败而不是日志里的明文。
class ApiClient {
  ApiClient({String? baseUrl, Dio? dio})
      : _baseUrl = baseUrl ?? defaultBaseUrl,
        _dio = dio ?? Dio() {
    _dio.options
      ..baseUrl = _baseUrl
      ..connectTimeout = const Duration(seconds: 10)
      ..receiveTimeout = const Duration(seconds: 20)
      ..headers['Accept'] = 'application/json';

    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (RequestOptions options, RequestInterceptorHandler handler) async {
          _assertNoSensitiveQuery(options);
          final token = await _storage.read(key: _tokenKey);
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          handler.next(options);
        },
      ),
    );
  }

  /// 生产/开发基址。域名与 PRD v2 §12.1 一致。
  static const String defaultBaseUrl = 'https://cw-tc.ccit-agent.cn';

  static const String _tokenKey = 'cw_tc_session';

  final String _baseUrl;
  final Dio _dio;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  Dio get raw => _dio;

  /// 健康检查（`GET /api/v1/health`）。
  Future<Map<String, dynamic>> health() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/health');
    return res.data ?? <String, dynamic>{};
  }

  /// 课程广场列表（`GET /api/v1/courses`，公开只读）。
  Future<List<Course>> courses({int? majorId}) async {
    final res = await _dio.get<Map<String, dynamic>>(
      '/api/v1/courses',
      queryParameters: majorId != null ? {'major_id': majorId} : null,
    );
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items
        .map((e) => Course.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// 禁止把疑似身份证号 / 手机号放进 URL。
  ///
  /// 这里刻意做成"运行时抛错"而不是静默脱敏：一旦有人这么写，
  /// **在开发期就要炸掉**，而不是等它进了服务器日志。
  static void _assertNoSensitiveQuery(RequestOptions options) {
    final query = options.uri.query;
    if (query.isEmpty) return;
    if (_sensitiveInQuery.hasMatch(query)) {
      throw StateError(
        '检测到敏感信息出现在 URL query 中（${options.uri.path}）：'
        '身份证号与手机号必须走 POST body（PRD v2 §9.2 / AGENTS.md 铁律 3）。',
      );
    }
  }

  static final RegExp _sensitiveInQuery =
      RegExp(r'(\b\d{17}[\dXx]\b)|(\b1[3-9]\d{9}\b)');
}
