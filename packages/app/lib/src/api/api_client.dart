import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import '../models/cert.dart';
import '../models/course.dart';
import '../models/exam.dart';
import '../models/lecture.dart';
import '../models/lecture_resource.dart';
import '../models/major.dart';
import '../models/verify_result.dart';

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
  static const String _phoneKey = 'cw_tc_phone';

  final String _baseUrl;
  final Dio _dio;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  Dio get raw => _dio;

  /// 健康检查（`GET /api/v1/health`）。
  Future<Map<String, dynamic>> health() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/health');
    return res.data ?? <String, dynamic>{};
  }

  /// 密码登录（`POST /api/v1/auth/login`）：返回 token 并写入安全存储。
  Future<String> login({required String phone, required String password}) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/api/v1/auth/login',
      data: <String, dynamic>{'phone': phone, 'password': password},
    );
    final data = res.data?['data'] as Map<String, dynamic>?;
    final token = data?['token'] as String? ?? '';
    if (token.isNotEmpty) {
      await _storage.write(key: _tokenKey, value: token);
      await _storage.write(key: _phoneKey, value: phone);
    }
    return token;
  }

  /// 注册（`POST /api/v1/auth/register`）：注册即登录，返回 token 并写入安全存储。
  Future<String> register({
    required String name,
    required String phone,
    required String idCard,
    required String password,
  }) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/api/v1/auth/register',
      data: <String, dynamic>{
        'name': name,
        'phone': phone,
        'id_card': idCard,
        'password': password,
      },
    );
    final data = res.data?['data'] as Map<String, dynamic>?;
    final token = data?['token'] as String? ?? '';
    if (token.isNotEmpty) {
      await _storage.write(key: _tokenKey, value: token);
      await _storage.write(key: _phoneKey, value: phone);
    }
    return token;
  }

  /// 登出：清除本地会话凭据。
  Future<void> logout() async {
    await _storage.delete(key: _tokenKey);
    await _storage.delete(key: _phoneKey);
  }

  /// 是否已登录（本地有会话凭据）。
  Future<bool> hasSession() async {
    final token = await _storage.read(key: _tokenKey);
    return token != null && token.isNotEmpty;
  }

  /// 已登录的手机号（登录/注册时写入，未登录返回 null）。
  Future<String?> storedPhone() async {
    final phone = await _storage.read(key: _phoneKey);
    return (phone == null || phone.isEmpty) ? null : phone;
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

  /// 专业分类列表（`GET /api/v1/majors`，公开只读，启用的）。
  Future<List<Major>> majors() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/majors');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items
        .map((e) => Major.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// 创建订单（`POST /api/v1/orders`，用课程码），返回 order_no。
  Future<String> createOrder(String courseCode) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/api/v1/orders',
      data: <String, dynamic>{'course_code': courseCode},
    );
    final data = res.data?['data'] as Map<String, dynamic>?;
    return data?['order_no'] as String? ?? '';
  }

  /// 模拟支付成功（`POST /orders/:orderNo/mock-pay`，仅支付渠道未接入时）。
  Future<void> mockPayOrder(String orderNo) async {
    await _dio.post<Map<String, dynamic>>('/api/v1/orders/$orderNo/mock-pay', data: <String, dynamic>{});
  }

  /// 我的学习权益（`GET /api/v1/me/entitlements`），返回已购课程码列表。
  Future<List<String>> myEntitlementCourseCodes() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/me/entitlements');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items
        .map((e) => e['course_code'] as String? ?? '')
        .where((c) => c.isNotEmpty)
        .toList();
  }

  /// 课程详情（`GET /api/v1/courses/:courseCode`，公开只读）。
  Future<Course> courseDetail(String courseCode) async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/courses/$courseCode');
    final data = res.data?['data'] as Map<String, dynamic>?;
    return Course.fromJson(data ?? const <String, dynamic>{});
  }

  /// 课程讲次列表（`GET /api/v1/courses/:courseCode/lectures`，公开只读）。
  Future<List<Lecture>> lectures(String courseCode) async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/courses/$courseCode/lectures');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items
        .map((e) => Lecture.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// 开考（`POST /exams`）：返回 session_id + 试卷（不含答案）。
  Future<ExamSession> startExam({required String courseCode, required String level}) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/api/v1/exams',
      data: <String, dynamic>{'course_code': courseCode, 'level': level},
    );
    final data = res.data?['data'] as Map<String, dynamic>?;
    final sessionId = (data?['session_id'] as num).toInt();
    final rawPaper = (data?['paper'] as List<dynamic>?) ?? const <dynamic>[];
    final paper = rawPaper
        .map((e) => ExamQuestion.fromJson(e as Map<String, dynamic>))
        .toList();
    return ExamSession(sessionId: sessionId, paper: paper);
  }

  /// 交卷判分（`POST /exams/:sessionID/submit`）：通过自动发证。
  Future<ExamResult> submitExam(int sessionId, Map<String, String> answers) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/api/v1/exams/$sessionId/submit',
      data: <String, dynamic>{'answers': answers},
    );
    final data = res.data?['data'] as Map<String, dynamic>?;
    return ExamResult.fromJson(data ?? const <String, dynamic>{});
  }

  /// 我的证书列表（`GET /me/certs`）。
  Future<List<Cert>> myCerts() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/me/certs');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items.map((e) => Cert.fromJson(e as Map<String, dynamic>)).toList();
  }

  /// 证书验真（`GET /verify/:certNo`，公开）。
  Future<VerifyResult> verifyCert(String certNo) async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/verify/$certNo');
    final data = res.data?['data'] as Map<String, dynamic>?;
    return VerifyResult.fromJson(data ?? const <String, dynamic>{});
  }

  /// 讲次资源清单（`GET /courses/:code/lectures/:no/resources`，含签名 URL）。
  Future<List<LectureResource>> lectureResources(String courseCode, int no) async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/courses/$courseCode/lectures/$no/resources');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items.map((e) => LectureResource.fromJson(e as Map<String, dynamic>)).toList();
  }

  /// 资源签名下载 URL（`GET /api/v1/assets/raw?key=&expires=&sig=`）。
  ///
  /// 后端校验签名与过期时间后直接从内容目录取文件（不剥 JSON 信封、返回原始字节）。
  /// 供网页端/系统浏览器直接打开二进制资源（课件/视频）时拼 URL 使用。
  String resourceUrl(LectureResource r) {
    final uri = Uri.parse(_baseUrl).replace(
      path: '/api/v1/assets/raw',
      queryParameters: <String, String>{
        'key': r.storageKey,
        'expires': '${r.expiresAt}',
        'sig': r.signature,
      },
    );
    return uri.toString();
  }

  /// 拉取文本类资源（讲义 Markdown / 代码 / HTML）原文，供端内阅读。
  Future<String> fetchResourceText(LectureResource r) async {
    final res = await _dio.get<String>(
      '/api/v1/assets/raw',
      queryParameters: <String, dynamic>{
        'key': r.storageKey,
        'expires': r.expiresAt,
        'sig': r.signature,
      },
    );
    return res.data ?? '';
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
