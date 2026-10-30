import 'package:cw_tc_app/src/api/api_client.dart';
import 'package:cw_tc_app/src/models/favorite.dart';
import 'package:cw_tc_app/src/pages/my_favorites_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

class _FavApi extends ApiClient {
  _FavApi(this.items);
  List<FavoriteCourse> items;
  bool removed = false;

  @override
  Future<List<FavoriteCourse>> myFavorites() async => items;

  @override
  Future<void> setFavorite(String courseCode, bool favorited) async {
    removed = true;
    items = items.where((c) => c.courseCode != courseCode).toList();
  }
}

void main() {
  testWidgets('shows favorites and removes', (tester) async {
    final api = _FavApi(<FavoriteCourse>[
      const FavoriteCourse(
        courseCode: 'C1',
        name: '课程一',
        priceCents: 1000,
        courseStatus: 'published',
        unlocked: false,
        createdAt: '2026-10-01T00:00:00Z',
      ),
    ]);
    await tester.pumpWidget(MaterialApp(home: MyFavoritesPage(api: api)));
    await tester.pumpAndSettle();
    expect(find.text('课程一'), findsOneWidget);
    await tester.tap(find.byIcon(Icons.favorite));
    await tester.pumpAndSettle();
    expect(api.removed, isTrue);
  });
}
