import 'package:material_3_expressive/material_3_expressive.dart';
import 'package:material_ui/material_ui.dart';

import '../api_client.dart';

/// Log browsers for the three admin tables.
class LogsPage extends StatefulWidget {
  const LogsPage({super.key});

  @override
  State<LogsPage> createState() => _LogsPageState();
}

class _LogsPageState extends State<LogsPage> {
  String _kind = 'credentials';
  final _user = TextEditingController();
  List<dynamic> _items = [];
  num _total = 0;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _user.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final q = <String, String>{'limit': '50'};
      if (_user.text.trim().isNotEmpty) q['user_id'] = _user.text.trim();
      final data =
          await api.get('/api/v1/admin/logs/$_kind', q) as Map<String, dynamic>;
      setState(() {
        _items = (data['items'] as List?) ?? [];
        _total = data['total'] as num? ?? 0;
      });
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String _rowText(Map<String, dynamic> r) {
    switch (_kind) {
      case 'credentials':
        return '${r['created_at'] ?? ''} · ${r['role'] ?? ''}/${r['permission'] ?? ''}'
            ' · ${r['key_type'] ?? '明文'} · ${r['ip'] ?? ''}';
      case 'updates':
        return '${r['created_at'] ?? ''} · ${r['resolved'] == true ? '成功' : '失败'}'
            ' · v${r['version_code'] ?? '?'} · ${r['key_type'] ?? '明文'} · ${r['ip'] ?? ''}';
      default:
        return '${r['created_at'] ?? ''} · ${r['action'] ?? ''} · ${r['ip'] ?? ''}';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          Row(
            children: [
              M3ESegmentedButton<String>(
                segments: const [
                  M3ESegment(value: 'credentials', label: '凭据'),
                  M3ESegment(value: 'updates', label: '解析'),
                  M3ESegment(value: 'audit', label: '审计'),
                ],
                selected: {_kind},
                onSelectionChanged: (v) {
                  if (v.isNotEmpty) {
                    setState(() => _kind = v.first);
                    _load();
                  }
                },
              ),
              const SizedBox(width: 12),
              SizedBox(
                width: 280,
                child: M3ETextField(
                  controller: _user,
                  label: '按 user_id 过滤',
                  showClearButton: true,
                ),
              ),
              const SizedBox(width: 8),
              M3EButton(
                style: M3EButtonStyle.filled,
                onPressed: _load,
                child: const Text('查询'),
              ),
              const Spacer(),
              Text('共 $_total 条'),
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
                          final r = _items[i] as Map<String, dynamic>;
                          return M3EListItem(
                            headline:
                                (r['user_id'] ?? '(匿名)').toString(),
                            supportingText: _rowText(r),
                          );
                        },
                      ),
          ),
        ],
      ),
    );
  }
}
