import 'package:material_3_expressive/material_3_expressive.dart';
import 'package:material_ui/material_ui.dart';

import '../api_client.dart';

/// Multi-IP session alerts (credential-sharing / theft signal).
class AlertsPage extends StatefulWidget {
  const AlertsPage({super.key});

  @override
  State<AlertsPage> createState() => _AlertsPageState();
}

class _AlertsPageState extends State<AlertsPage> {
  final _window = TextEditingController(text: '24');
  final _minIps = TextEditingController(text: '3');
  List<dynamic> _alerts = [];
  bool _busy = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _window.dispose();
    _minIps.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final data = await api.get('/api/v1/admin/alerts/sessions', {
        'windowH': _window.text.trim().isEmpty ? '24' : _window.text.trim(),
        'minIps': _minIps.text.trim().isEmpty ? '3' : _minIps.text.trim(),
      }) as Map<String, dynamic>;
      setState(() => _alerts = (data['alerts'] as List?) ?? []);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
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
              Text('会话告警',
                  style: M3ETheme.of(context).typography.emphasized.headlineSmall),
              const SizedBox(width: 16),
              SizedBox(
                width: 140,
                child: M3ETextField(controller: _window, label: '窗口(小时)'),
              ),
              const SizedBox(width: 8),
              SizedBox(
                width: 140,
                child: M3ETextField(controller: _minIps, label: '最少IP数'),
              ),
              const SizedBox(width: 8),
              M3EButton(
                style: M3EButtonStyle.filled,
                onPressed: _load,
                child: const Text('查询'),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Expanded(
            child: _busy
                ? const Center(child: M3ELoadingIndicator())
                : _error != null
                    ? Center(child: Text(_error!))
                    : _alerts.isEmpty
                        ? const Center(child: Text('无告警'))
                        : M3EList(
                            itemCount: _alerts.length,
                            itemBuilder: (context, i) {
                              final a = _alerts[i] as Map<String, dynamic>;
                              final ips = ((a['ips'] as List?) ?? []).join(', ');
                              return M3ECard(
                                variant: M3ECardVariant.outlined,
                                headline: Text(
                                    '${a['userId']} · ${a['ipCount']} 个 IP'),
                                supportingText: Text(
                                    '$ips\n最近出现 ${a['lastSeen'] ?? ''}'),
                              );
                            },
                          ),
          ),
          if (!_busy && _error == null)
            Text('NAT 出口会产生误报，阈值按实际调整',
                style: TextStyle(color: scheme.onSurfaceVariant)),
        ],
      ),
    );
  }
}
