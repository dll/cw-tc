import 'dart:convert';

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
        onRequest:
            (RequestOptions options, RequestInterceptorHandler handler) async {
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
  Future<String> login(
      {required String phone, required String password}) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '/api/v1/auth/login',
      data: <String, dynamic>{'phone': phone, 'password': password},
    );
    _ensureSuccess(res.data);
    final data = res.data?['data'] as Map<String, dynamic>?;
    final token = data?['token'] as String? ?? '';
    if (token.isEmpty) throw Exception('登录响应缺少会话令牌');
    if (token.isNotEmpty) {
      await _storage.write(key: _tokenKey, value: token);
      await _storage.write(key: _phoneKey, value: phone);
    }
    return token;
  }

  /// 注册（`POST /api/v1/auth/register`）：提交后等待运营审核，不建立会话。
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
    _ensureSuccess(res.data);
    final data = res.data?['data'] as Map<String, dynamic>?;
    final token = data?['token'] as String? ?? '';
    if (token.isNotEmpty) {
      await _storage.write(key: _tokenKey, value: token);
      await _storage.write(key: _phoneKey, value: phone);
    }
    return token;
  }

  void _ensureSuccess(Map<String, dynamic>? body) {
    final code = body?['code'];
    if (code != 0) {
      throw Exception((body?['msg'] as String?) ?? '请求失败（错误码 $code）');
    }
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
  ///
  /// 支持 `majorId` / `line`(main|aux) / `q`(关键词) 与分页（`pageSize` 上限 100）。
  Future<CoursePage> courses({
    int? majorId,
    String? line,
    String? q,
    int page = 1,
    int pageSize = 20,
  }) async {
    final query = <String, dynamic>{'page': page, 'page_size': pageSize};
    if (majorId != null) query['major_id'] = majorId;
    if (line != null && line.isNotEmpty) query['line'] = line;
    if (q != null && q.trim().isNotEmpty) query['q'] = q.trim();
    final res = await _dio.get<Map<String, dynamic>>(
      '/api/v1/courses',
      queryParameters: query,
    );
    return CoursePage.fromJson(res.data?['data'] as Map<String, dynamic>?);
  }

  /// 专业分类列表（`GET /api/v1/majors`，公开只读，启用的）。
  Future<List<Major>> majors() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/majors');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items.map((e) => Major.fromJson(e as Map<String, dynamic>)).toList();
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

  /// 生成与当前订单绑定的支付宝当面付二维码内容。
  Future<String> alipayPrecreate(String orderNo) async {
    final res = await _dio
        .post<Map<String, dynamic>>('/api/v1/orders/$orderNo/alipay/precreate');
    final data = res.data?['data'] as Map<String, dynamic>?;
    return data?['qr_code'] as String? ?? '';
  }

  Future<String> orderStatus(String orderNo) async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/orders/$orderNo');
    final data = res.data?['data'] as Map<String, dynamic>?;
    return data?['status'] as String? ?? '';
  }

  /// 模拟支付成功（`POST /orders/:orderNo/mock-pay`，仅支付渠道未接入时）。
  Future<void> mockPayOrder(String orderNo) async {
    await _dio.post<Map<String, dynamic>>('/api/v1/orders/$orderNo/mock-pay',
        data: <String, dynamic>{});
  }

  /// 我的学习权益来源（`course_code -> order|free|grant`）。
  Future<Map<String, String>> myEntitlementSources() async {
    final res = await _dio.get<Map<String, dynamic>>('/api/v1/me/entitlements');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    final sources = <String, String>{};
    for (final item in items) {
      final row = item as Map<String, dynamic>?;
      final code = row?['course_code'] as String? ?? '';
      if (code.isEmpty) continue;
      sources[code] = row?['source'] as String? ?? '';
    }
    return sources;
  }

  /// 我的学习权益（`GET /api/v1/me/entitlements`），返回已购课程码列表。
  Future<List<String>> myEntitlementCourseCodes() async =>
      (await myEntitlementSources()).keys.toList();

  /// 课程详情（`GET /api/v1/courses/:courseCode`，公开只读）。
  Future<Course> courseDetail(String courseCode) async {
    final res =
        await _dio.get<Map<String, dynamic>>('/api/v1/courses/$courseCode');
    final data = res.data?['data'] as Map<String, dynamic>?;
    return Course.fromJson(data ?? const <String, dynamic>{});
  }

  /// 课程讲次列表（`GET /api/v1/courses/:courseCode/lectures`，公开只读）。
  Future<List<Lecture>> lectures(String courseCode) async {
    final res = await _dio
        .get<Map<String, dynamic>>('/api/v1/courses/$courseCode/lectures');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items
        .map((e) => Lecture.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// 开考（`POST /exams`）：返回 session_id + 试卷（不含答案）。
  Future<ExamSession> startExam(
      {required String courseCode, required String level}) async {
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
  Future<ExamResult> submitExam(
      int sessionId, Map<String, String> answers) async {
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
  Future<List<LectureResource>> lectureResources(
      String courseCode, int no) async {
    final res = await _dio.get<Map<String, dynamic>>(
        '/api/v1/courses/$courseCode/lectures/$no/resources');
    final data = res.data?['data'] as Map<String, dynamic>?;
    final items = (data?['items'] as List<dynamic>?) ?? const <dynamic>[];
    return items
        .map((e) => LectureResource.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  String _lecturePath(String code, int no) =>
      '/api/v1/courses/$code/lectures/$no';

  Future<List<Map<String, dynamic>>> lecturePractice(
      String code, int no) async {
    final res = await _dio
        .get<Map<String, dynamic>>('${_lecturePath(code, no)}/practice');
    return ((res.data?['data'] as Map<String, dynamic>?)?['items']
                as List<dynamic>? ??
            const [])
        .cast<Map<String, dynamic>>();
  }

  Future<Map<String, dynamic>> submitLectureAnswer(
      String code, int no, int questionId, String answer) async {
    final res = await _dio.post<Map<String, dynamic>>(
      '${_lecturePath(code, no)}/practice/$questionId/answers',
      data: <String, dynamic>{'answer': answer},
    );
    return res.data?['data'] as Map<String, dynamic>? ?? <String, dynamic>{};
  }

  Future<List<Map<String, dynamic>>> lectureTopics(String code, int no) async {
    final res = await _dio
        .get<Map<String, dynamic>>('${_lecturePath(code, no)}/topics');
    return ((res.data?['data'] as Map<String, dynamic>?)?['items']
                as List<dynamic>? ??
            const [])
        .cast<Map<String, dynamic>>();
  }

  Future<void> postLectureTopic(String code, int no, String body) async {
    await _dio.post<Map<String, dynamic>>('${_lecturePath(code, no)}/topics',
        data: <String, dynamic>{'body': body});
  }

  Future<List<Map<String, dynamic>>> lectureReplies(
      String code, int no, int topicId) async {
    final res = await _dio.get<Map<String, dynamic>>(
        '${_lecturePath(code, no)}/topics/$topicId/replies');
    return ((res.data?['data'] as Map<String, dynamic>?)?['items']
                as List<dynamic>? ??
            const [])
        .cast<Map<String, dynamic>>();
  }

  Future<void> postLectureReply(
      String code, int no, int topicId, String body) async {
    await _dio.post<Map<String, dynamic>>(
        '${_lecturePath(code, no)}/topics/$topicId/replies',
        data: <String, dynamic>{'body': body});
  }

  Future<void> likeLectureTopic(String code, int no, int topicId) async {
    await _dio.post<Map<String, dynamic>>(
        '${_lecturePath(code, no)}/topics/$topicId/like');
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

  /// 文件服务回退地址可能是站内相对路径，播放器必须收到完整 URL。
  String mediaUrl(String url) => Uri.parse(_baseUrl).resolve(url).toString();

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

  /// 拉取二进制资源（课件 PPTX）为 base64，供端内渲染。
  Future<String> fetchResourceBase64(LectureResource r) async {
    final res = await _dio.get<List<int>>(
      '/api/v1/assets/raw',
      queryParameters: <String, dynamic>{
        'key': r.storageKey,
        'expires': r.expiresAt,
        'sig': r.signature,
      },
      options: Options(responseType: ResponseType.bytes),
    );
    return base64Encode(res.data ?? const <int>[]);
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
