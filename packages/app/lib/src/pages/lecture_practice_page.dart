import 'package:flutter/material.dart';

import '../api/api_client.dart';

/// 每讲开放练习：提交自答后展示上游参考解答，不参与课程总测评。
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
  final Map<int, String> references = <int, String>{};
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
      final reference = await widget.api
          .submitLectureAnswer(widget.courseCode, widget.no, id, answer);
      if (mounted) setState(() => references[id] = reference);
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
        appBar: AppBar(title: Text('第 ${widget.no} 讲测评')),
        body: FutureBuilder<List<Map<String, dynamic>>>(
          future: questions,
          builder: (context, snapshot) {
            if (!snapshot.hasData) {
              return Center(child: Text(snapshot.hasError ? '练习加载失败' : '加载中…'));
            }
            final items = snapshot.data!;
            if (items.isEmpty) return const Center(child: Text('本讲暂无可抽取的思考题'));
            return ListView(
                padding: const EdgeInsets.all(16),
                children: <Widget>[
                  const Text('根据本讲思考题作答。提交后可对照参考解答；本讲练习不计入课程总测评和证书。'),
                  for (final q in items)
                    Card(
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
                                controller: answers.putIfAbsent(
                                    q['id'] as int, TextEditingController.new),
                                maxLines: 4,
                                decoration: const InputDecoration(
                                    border: OutlineInputBorder(),
                                    hintText: '写下你的答案')),
                            const SizedBox(height: 8),
                            FilledButton(
                                onPressed: saving.contains(q['id'])
                                    ? null
                                    : () => submit(q['id'] as int),
                                child: const Text('提交并查看参考')),
                            if (references.containsKey(q['id']))
                              Padding(
                                padding: const EdgeInsets.only(top: 12),
                                child: Text(references[q['id']]!.isEmpty
                                    ? '本题暂无参考解答，请结合讲义核对。'
                                    : '参考解答：\n${references[q['id']]}'),
                              ),
                          ]),
                    )),
                ]);
          },
        ),
      );
}
