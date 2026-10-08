import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/lecture_resource.dart';
import 'package:cw_tc_app/src/pages/audio_player_page.dart';
import 'package:cw_tc_app/src/pages/learn_page.dart';
import 'package:cw_tc_app/src/pages/video_player_page.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// 假客户端：覆盖资源取数与文本拉取，不发真实网络请求（避免 pending timer）。
class _FakeApiClient extends ApiClient {
  @override
  Future<List<LectureResource>> lectureResources(String courseCode, int no) async {
    return const <LectureResource>[
      LectureResource(
        kind: 'markdown',
        title: '讲义',
        storageKey: 'AI化教学创新03-CF/01_你好世界/README.md',
        sizeBytes: 2242,
        expiresAt: 0,
        signature: 's',
      ),
      LectureResource(
        kind: 'video',
        title: '视频',
        storageKey: 'AI化教学创新03-CF/01_你好世界/video/hello.mp4',
        sizeBytes: 60899065,
        expiresAt: 0,
        signature: 's',
        url: 'https://example.com/hello.mp4?sign=abc',
      ),
      LectureResource(
        kind: 'audio',
        title: '音频',
        storageKey: 'AI化教学创新03-CF/01_你好世界/audio_01.wav',
        sizeBytes: 102400,
        expiresAt: 0,
        signature: 's',
        url: 'https://example.com/audio_01.wav?sign=abc',
      ),
    ];
  }

  @override
  Future<String> fetchResourceText(LectureResource r) async => '# 讲义内容';
}

/// 未购课（403）假客户端。
class _ForbiddenApiClient extends ApiClient {
  @override
  Future<List<LectureResource>> lectureResources(String courseCode, int no) async {
    throw DioException(
      requestOptions: RequestOptions(path: '/api/v1/courses/$courseCode/lectures/$no/resources'),
      response: Response(requestOptions: RequestOptions(path: 'x'), statusCode: 403),
    );
  }
}

void main() {
  testWidgets('学习页展示资源清单（文件名 + 类型 + 大小）', (WidgetTester tester) async {
    final api = _FakeApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    expect(find.text('README.md'), findsOneWidget);
    expect(find.text('讲义 · 2.2 KB'), findsOneWidget);
    expect(find.text('hello.mp4'), findsOneWidget);
    expect(find.text('视频 · 58.1 MB'), findsOneWidget);
  });

  testWidgets('点击文本资源进入阅读页', (WidgetTester tester) async {
    final api = _FakeApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('README.md'));
    await tester.pumpAndSettle();

    expect(find.text('# 讲义内容'), findsOneWidget);
  });

  testWidgets('点击视频资源进入播放页', (WidgetTester tester) async {
    final api = _FakeApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('hello.mp4'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(VideoPlayerPage), findsOneWidget);
  });

  testWidgets('点击音频资源进入播放页', (WidgetTester tester) async {
    final api = _FakeApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('audio_01.wav'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.byType(AudioPlayerPage), findsOneWidget);
  });

  testWidgets('未购课（403）显示购买提示', (WidgetTester tester) async {
    final api = _ForbiddenApiClient();
    await tester.pumpWidget(
      MaterialApp(home: LearnPage(courseCode: 'C1', no: 1, api: api)),
    );
    await tester.pumpAndSettle();

    expect(find.text('请先购买 / 解锁本课程'), findsOneWidget);
  });
}
