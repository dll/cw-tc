import 'package:cw_tc_app/src/app.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('首页渲染标题与入口按钮', (WidgetTester tester) async {
    await tester.pumpWidget(const CwTcApp());
    await tester.pumpAndSettle();

    expect(find.text('第三课堂'), findsWidgets);
    expect(find.text('进入课程广场'), findsOneWidget);
    expect(find.text('证书验真'), findsOneWidget);
  });

  testWidgets('点击进入课程广场', (WidgetTester tester) async {
    await tester.pumpWidget(const CwTcApp());
    await tester.pumpAndSettle();

    await tester.tap(find.text('进入课程广场'));
    await tester.pumpAndSettle();

    expect(find.text('课程广场'), findsWidgets);
    // 不编数：未接入数据源时应显示显式提示，而不是 "0 门课程"
    expect(find.text('课程列表尚未接入数据源（M1）'), findsOneWidget);
  });
}
