import 'package:flutter_test/flutter_test.dart';
import 'package:cw_tc_app/src/pages/resource_viewer_page.dart';

void main() {
  test('HTML 相对音频使用授权基址', () {
    const html = '<!doctype html><html><head><base href="/old/">'
        '</head><body><audio src="assets/audio/step_00.mp3"></audio></body></html>';
    const base = 'https://cw-tc.ccit-agent.cn/api/v1/assets/inline/1/2/3/sig/';
    final result = htmlWithBase(html, base);

    expect(result.indexOf('<base href="$base">'),
        greaterThan(result.indexOf('<head>')));
    expect(result.indexOf('<base href="$base">'),
        lessThan(result.indexOf('/old/')));
    expect(result, contains('assets/audio/step_00.mp3'));
  });
}
