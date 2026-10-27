import 'package:flutter/material.dart';

import '../api/api_client.dart';

/// 讲次讨论按点赞数排序；任何访客可看，登录后可发帖、评论、点赞。
class LectureDiscussionPage extends StatefulWidget {
  const LectureDiscussionPage(
      {super.key,
      required this.courseCode,
      required this.no,
      required this.api});
  final String courseCode;
  final int no;
  final ApiClient api;

  @override
  State<LectureDiscussionPage> createState() => _LectureDiscussionPageState();
}

class _LectureDiscussionPageState extends State<LectureDiscussionPage> {
  late Future<List<Map<String, dynamic>>> topics =
      widget.api.lectureTopics(widget.courseCode, widget.no);

  void reload() => setState(
      () => topics = widget.api.lectureTopics(widget.courseCode, widget.no));

  Future<void> compose() async {
    final body = await _askText(context, '发起讨论');
    if (body == null) return;
    try {
      await widget.api.postLectureTopic(widget.courseCode, widget.no, body);
      reload();
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('发布失败，请登录并检查内容')));
      }
    }
  }

  Future<void> like(int id) async {
    try {
      await widget.api.likeLectureTopic(widget.courseCode, widget.no, id);
      reload();
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('点赞失败，请先登录')));
      }
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: Text('第 ${widget.no} 讲讨论')),
        floatingActionButton: FloatingActionButton.extended(
            onPressed: compose,
            icon: const Icon(Icons.edit),
            label: const Text('发起讨论')),
        body: FutureBuilder<List<Map<String, dynamic>>>(
            future: topics,
            builder: (context, snapshot) {
              if (!snapshot.hasData) {
                return Center(
                    child: Text(snapshot.hasError ? '讨论加载失败' : '加载中…'));
              }
              if (snapshot.data!.isEmpty) {
                return const Center(child: Text('暂无讨论，来发表第一条吧'));
              }
              return RefreshIndicator(
                  onRefresh: () async {
                    reload();
                    await topics;
                  },
                  child: ListView.builder(
                    itemCount: snapshot.data!.length,
                    itemBuilder: (context, index) {
                      final topic = snapshot.data![index];
                      final id = topic['id'] as int;
                      return Card(
                          child: Column(children: <Widget>[
                        ListTile(
                            title: Text(topic['body'] as String? ?? ''),
                            subtitle:
                                Text(topic['author'] as String? ?? '学员***'),
                            onTap: () => Navigator.of(context).push(
                                MaterialPageRoute<void>(
                                    builder: (_) => _RepliesPage(
                                        api: widget.api,
                                        code: widget.courseCode,
                                        no: widget.no,
                                        topicId: id,
                                        body:
                                            topic['body'] as String? ?? '')))),
                        Row(children: <Widget>[
                          TextButton.icon(
                              onPressed: () => like(id),
                              icon: const Icon(Icons.thumb_up_outlined),
                              label: Text('${topic['likes'] ?? 0}')),
                          TextButton.icon(
                              onPressed: () => Navigator.of(context).push(
                                  MaterialPageRoute<void>(
                                      builder: (_) => _RepliesPage(
                                          api: widget.api,
                                          code: widget.courseCode,
                                          no: widget.no,
                                          topicId: id,
                                          body:
                                              topic['body'] as String? ?? ''))),
                              icon: const Icon(Icons.comment_outlined),
                              label: const Text('评论')),
                        ]),
                      ]));
                    },
                  ));
            }),
      );
}

class _RepliesPage extends StatefulWidget {
  const _RepliesPage(
      {required this.api,
      required this.code,
      required this.no,
      required this.topicId,
      required this.body});
  final ApiClient api;
  final String code;
  final int no;
  final int topicId;
  final String body;
  @override
  State<_RepliesPage> createState() => _RepliesPageState();
}

class _RepliesPageState extends State<_RepliesPage> {
  late Future<List<Map<String, dynamic>>> replies =
      widget.api.lectureReplies(widget.code, widget.no, widget.topicId);

  Future<void> compose() async {
    final body = await _askText(context, '发表评论');
    if (body == null) return;
    try {
      await widget.api
          .postLectureReply(widget.code, widget.no, widget.topicId, body);
      if (mounted) {
        setState(() => replies =
            widget.api.lectureReplies(widget.code, widget.no, widget.topicId));
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(const SnackBar(content: Text('评论失败，请登录并检查内容')));
      }
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('评论')),
        floatingActionButton: FloatingActionButton.extended(
            onPressed: compose,
            icon: const Icon(Icons.reply),
            label: const Text('发表评论')),
        body: FutureBuilder<List<Map<String, dynamic>>>(
            future: replies,
            builder: (context, snapshot) {
              if (!snapshot.hasData) {
                return Center(
                    child: Text(snapshot.hasError ? '评论加载失败' : '加载中…'));
              }
              return ListView(children: <Widget>[
                ListTile(title: Text(widget.body)),
                const Divider(),
                for (final reply in snapshot.data!)
                  ListTile(
                      title: Text(reply['body'] as String? ?? ''),
                      subtitle: Text(reply['author'] as String? ?? '学员***')),
              ]);
            }),
      );
}

Future<String?> _askText(BuildContext context, String title) async {
  final controller = TextEditingController();
  try {
    return await showDialog<String>(
        context: context,
        builder: (context) => AlertDialog(
              title: Text(title),
              content: TextField(
                  controller: controller,
                  maxLines: 5,
                  maxLength: 2000,
                  decoration:
                      const InputDecoration(hintText: '请勿发布个人信息或测评题目、答案')),
              actions: <Widget>[
                TextButton(
                    onPressed: () => Navigator.pop(context),
                    child: const Text('取消')),
                FilledButton(
                    onPressed: () {
                      final value = controller.text.trim();
                      if (value.isNotEmpty) Navigator.pop(context, value);
                    },
                    child: const Text('发布')),
              ],
            ));
  } finally {
    controller.dispose();
  }
}
