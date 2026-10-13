import 'package:material_3_expressive/material_3_expressive.dart';
import 'package:material_ui/material_ui.dart';

import '../api_client.dart';

class VersionsPage extends StatefulWidget {
  const VersionsPage({super.key});

  @override
  State<VersionsPage> createState() => _VersionsPageState();
}

class _VersionsPageState extends State<VersionsPage> {
  List<dynamic> _items = [];
  bool _busy = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final data =
          await api.get('/api/v1/admin/versions', {'limit': '50'}) as Map<String, dynamic>;
      setState(() => _items = (data['items'] as List?) ?? []);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _publish() async {
    final code = TextEditingController();
    final name = TextEditingController();
    final url = TextEditingController();
    bool force = false;
    final ok = await M3EDialog.show<bool>(
      context,
      dialog: M3EDialog(
        title: '发布新版本',
        content: StatefulBuilder(
          builder: (context, setRow) => Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              M3ETextField(controller: code, label: 'Version code（整数）'),
              M3ETextField(controller: name, label: 'Version name'),
              M3ETextField(controller: url, label: 'Target URL（https）'),
              M3ECheckbox(
                value: force,
                label: const Text('强制更新'),
                onChanged: (v) => setRow(() => force = v ?? false),
              ),
            ],
          ),
        ),
        actions: [
          M3EButton.text(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('取消')),
          M3EButton(
              style: M3EButtonStyle.filled,
              onPressed: () => Navigator.pop(context, true),
              child: const Text('发布')),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    try {
      await api.post('/api/v1/admin/versions', {
        'versionCode': int.parse(code.text.trim()),
        'versionName': name.text.trim(),
        'forceUpdate': force,
        'targetUrl': url.text.trim(),
      });
      _load();
      M3ESnackbar.show(context, message: '已发布');
    } on ApiException catch (e) {
      M3ESnackbar.show(context, message: '发布失败：${e.message}');
    }
  }

  Future<void> _deleteLatest(int code) async {
    final ok = await M3EDialog.show<bool>(
      context,
      dialog: M3EDialog(
        title: '删除最新版并回滚？',
        content: Text('将删除 versionCode=$code，上一版自动成为最新。'),
        actions: [
          M3EButton.text(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('取消')),
          M3EButton(
              style: M3EButtonStyle.filled,
              onPressed: () => Navigator.pop(context, true),
              child: const Text('删除')),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    try {
      await api.delete('/api/v1/admin/versions/$code');
      _load();
    } on ApiException catch (e) {
      M3ESnackbar.show(context, message: '删除失败：${e.message}');
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = M3ETheme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          Row(
            children: [
              Text('版本列表',
                  style: M3ETheme.of(context).typography.emphasized.headlineSmall),
              const Spacer(),
              M3EButton(
                style: M3EButtonStyle.filled,
                onPressed: _publish,
                child: const Text('发布新版'),
              ),
              const SizedBox(width: 8),
              M3EButton.text(onPressed: _load, child: const Text('刷新')),
            ],
          ),
          const SizedBox(height: 12),
          Expanded(
            child: _busy
                ? const Center(child: M3ELoadingIndicator())
                : _error != null
                    ? Center(child: Text(_error!))
                    : M3EList(
                        itemCount: _items.length,
                        itemBuilder: (context, i) {
                          final v = _items[i] as Map<String, dynamic>;
                          final latest = i == 0;
                          return M3EListItem(
                            headline:
                                '${v['version_name']} (${v['version_code']})${latest ? ' · 最新' : ''}',
                            supportingText:
                                '${v['force_update'] == true ? '强制更新 · ' : ''}${v['target_url'] ?? ''}',
                            trailing: latest
                                ? M3EButton.text(
                                    onPressed: () => _deleteLatest(
                                        (v['version_code'] as num).toInt()),
                                    child: Text('删除',
                                        style: TextStyle(color: scheme.error)),
                                  )
                                : null,
                          );
                        },
                      ),
          ),
        ],
      ),
    );
  }
}
