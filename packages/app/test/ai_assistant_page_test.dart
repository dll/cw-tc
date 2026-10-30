import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/ai_answer.dart';
import 'package:cw_tc_app/src/pages/ai_assistant_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _AiApi extends ApiClient {
  @override
  Future<AiAnswer> askAI({
    required String courseCode,
    required int lectureNo,
    required String question,
  }) async =>
      const AiAnswer(
        answer: '核心概念是分层设计。\n\n来源：第1讲《讲义》',
        outcome: 'answered',
        citations: <AiCitation>[AiCitation(lectureNo: 1, title: '讲义')],
        model: 'deepseek-chat',
        remaining: 19,
      );
}

void main() {
  testWidgets('asks ai and shows citations', (tester) async {
    await tester.pumpWidget(MaterialApp(
        home: AiAssistantPage(
            courseCode: 'C1', lectureNo: 1, api: _AiApi())));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '这一讲讲什么？');
    await tester.tap(find.byIcon(Icons.send));
    await tester.pumpAndSettle();
    expect(find.textContaining('核心概念是分层设计'), findsOneWidget);
    expect(find.text('第 1 讲 · 讲义'), findsOneWidget);
    expect(find.textContaining('今日剩余 19 次'), findsOneWidget);
  });
}
