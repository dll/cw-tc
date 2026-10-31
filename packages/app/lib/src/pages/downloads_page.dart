import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/download_grant.dart';

/// 离线下载：只对已解锁课程签发授权，退款后由服务端置为失效（PRD §7.4 B-11）。
class DownloadsPage extends StatefulWidget {
  const DownloadsPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<DownloadsPage> createState() => _DownloadsPageState();
}

class _DownloadsPageState extends State<DownloadsPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<DownloadGrant> _items = <DownloadGrant>[];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final items = await _api.downloads();
      if (mounted) {
        setState(() {
          _items = items;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _items = <DownloadGrant>[];
          _loading = false;
        });
      }
    }
  }

  Future<void> _add() async {
    final course = TextEditingController();
    final lecture = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('申请离线下载'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            TextField(controller: course, decoration: const InputDecoration(labelText: '课程编码', border: OutlineInputBorder())),
            const SizedBox(height: 12),
            TextField(controller: lecture, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '讲次号', border: OutlineInputBorder())),
          ],
        ),
        actions: <Widget>[
          TextButton(onPressed: () => Navigator.of(dialogContext).pop(false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.of(dialogContext).pop(true), child: const Text('申请')),
        ],
      ),
    );
    final code = course.text.trim();
    final no = int.tryParse(lecture.text.trim()) ?? 0;
    course.dispose();
    lecture.dispose();
    if (ok != true || code.isEmpty || no < 1) return;
    try {
      await _api.requestDownload(code, no);
      if (mounted) await _load();
    } catch (_) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('申请失败：请确认已解锁该课程')));
    }
  }

  Future<void> _delete(DownloadGrant item) async {
    try {
      await _api.deleteDownload(item.lectureNo, courseCode: item.courseCode);
      if (mounted) await _load();
    } catch (_) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('删除失败')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('离线下载')),
      floatingActionButton: FloatingActionButton(onPressed: _add, child: const Icon(Icons.download_outlined)),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _items.isEmpty
              ? const Center(child: Text('暂无离线内容'))
              : RefreshIndicator(
                  onRefresh: _load,
                  child: ListView.separated(
                    padding: const EdgeInsets.all(12),
                    itemCount: _items.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 8),
                    itemBuilder: (context, i) {
                      final item = _items[i];
                      return Card(
                        child: ListTile(
                          leading: Icon(item.usable ? Icons.offline_pin : Icons.error_outline),
                          title: Text('${item.courseName} · 第 ${item.lectureNo} 讲'),
                          subtitle: Text('${item.lectureTitle}\n${item.statusLabel}'),
                          isThreeLine: true,
                          trailing: IconButton(
                            tooltip: '删除下载',
                            icon: const Icon(Icons.delete_outline),
                            onPressed: () => _delete(item),
                          ),
                        ),
                      );
                    },
                  ),
                ),
    );
  }
}
