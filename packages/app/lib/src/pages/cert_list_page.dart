import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/cert.dart';

/// 我的证书（PRD v2 §7.8 H）：已发证书列表，含状态（有效/已撤销）。
class CertListPage extends StatefulWidget {
  const CertListPage({super.key, this.api});

  final ApiClient? api;

  @override
  State<CertListPage> createState() => _CertListPageState();
}

class _CertListPageState extends State<CertListPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<List<Cert>> _future;

  @override
  void initState() {
    super.initState();
    _future = _api.myCerts();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('我的证书')),
      body: FutureBuilder<List<Cert>>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<List<Cert>> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return const Center(child: Text('证书加载失败（请先登录）'));
          }
          final certs = snapshot.data ?? const <Cert>[];
          if (certs.isEmpty) {
            return const Center(child: Text('暂无证书，通过测评后领取'));
          }
          return ListView.separated(
            padding: const EdgeInsets.all(12),
            itemCount: certs.length,
            separatorBuilder: (_, __) => const SizedBox(height: 8),
            itemBuilder: (BuildContext context, int i) {
              final cert = certs[i];
              final valid = cert.status == 'valid';
              return Card(
                margin: EdgeInsets.zero,
                child: ListTile(
                  leading: Icon(Icons.verified, color: valid ? theme.colorScheme.primary : theme.disabledColor),
                  title: Text(cert.courseName),
                  subtitle: Text('${cert.certNo} · ${cert.level} · ${cert.score} 分'),
                  trailing: Chip(label: Text(valid ? '有效' : '已撤销')),
                ),
              );
            },
          );
        },
      ),
    );
  }
}
