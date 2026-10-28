import 'package:flutter/material.dart';

import '../api/api_client.dart';

/// 每讲思考题练习：作答后自动评分并展示上游参考答案，不参与课程总测评。
class LecturePracticePage extends StatefulWidget {
  const LecturePracticePage(
      {super.key,
      required this.courseCode,
      required this.no,
      required this.api});

  final String courseCode;
  final int no;
  final ApiClient api;

  @override
  State<LecturePracticePage> createState() => _LecturePracticePageState();
}

class _LecturePracticePageState extends State<LecturePracticePage> {
  late Future<List<Map<String, dynamic>>> questions =
      widget.api.lecturePractice(widget.courseCode, widget.no);
  final Map<int, TextEditingController> answers =
      <int, TextEditingController>{};
  final Map<int, Map<String, dynamic>> results = <int, Map<String, dynamic>>{};
  final Set<int> saving = <int>{};

  @override
  void dispose() {
    for (final controller in answers.values) {
      controller.dispose();
    }
    super.dispose();
  }

  Future<void> submit(int id) async {
    final answer = answers[id]?.text.trim() ?? '';
    if (answer.isEmpty) return;
    setState(() => saving.add(id));
    try {
      final result = await widget.api
          .submitLectureAnswer(widget.courseCode, widget.no, id, answer);
      if (mounted) setState(() => results[id] = result);
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('保存失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => saving.remove(id));
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: Text('第 ${widget.no} 讲练习')),
        body: FutureBuilder<List<Map<String, dynamic>>>(
          future: questions,
          builder: (context, snapshot) {
            if (!snapshot.hasData) {
              return Center(child: Text(snapshot.hasError ? '练习加载失败' : '加载中…'));
            }
            final items = snapshot.data!;
            if (items.isEmpty) {
              return const Center(child: Text('本讲暂无可抽取的思考题'));
            }
            return ListView(
                padding: const EdgeInsets.all(16),
                children: <Widget>[
                  const Text('逐题作答后自动评分，并给出参考答案。本讲练习不计入课程总测评和证书。'),
                  const SizedBox(height: 12),
                  for (final q in items) _questionCard(context, q),
                ]);
          },
        ),
      );

  Widget _questionCard(BuildContext context, Map<String, dynamic> q) {
    final id = q['id'] as int;
    final result = results[id];
    final graded = result?['graded'] == true;
    final score = (result?['score'] as num?)?.toInt();
    final reference = result?['reference_answer'] as String? ?? '';
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(q['prompt'] as String? ?? '',
                style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            Text('依据：${q['evidence_ref'] ?? ''}',
                style: Theme.of(context).textTheme.bodySmall),
            const SizedBox(height: 8),
            TextField(
              controller:
                  answers.putIfAbsent(id, TextEditingController.new),
              maxLines: 4,
              decoration: const InputDecoration(
                  border: OutlineInputBorder(), hintText: '写下你的答案'),
            ),
            const SizedBox(height: 8),
            Row(
              children: <Widget>[
                FilledButton(
                  onPressed: saving.contains(id) ? null : () => submit(id),
                  child: Text(result == null ? '提交并评分' : '重新提交'),
                ),
                if (graded && score != null) ...<Widget>[
                  const SizedBox(width: 12),
                  Chip(
                    avatar: const Icon(Icons.verified_outlined, size: 18),
                    label: Text('自动评分 $score 分'),
                    backgroundColor:
                        Theme.of(context).colorScheme.secondaryContainer,
                  ),
                ],
              ],
            ),
            if (result != null)
              Padding(
                padding: const EdgeInsets.only(top: 12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    if (graded)
                      Text('评分依据：与参考答案的关键词重合度，仅供自查。',
                          style: Theme.of(context).textTheme.bodySmall),
                    if (!graded)
                      Text('本题为开放题，暂无自动评分，请对照课程资料自评。',
                          style: Theme.of(context).textTheme.bodySmall),
                    const SizedBox(height: 6),
                    SelectableText(reference.isEmpty
                        ? '本题暂无参考答案，请结合讲义核对。'
                        : '参考答案：\n$reference'),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}
