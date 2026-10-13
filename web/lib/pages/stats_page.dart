import 'package:material_3_expressive/material_3_expressive.dart';
import 'package:material_ui/material_ui.dart';

import '../api_client.dart';

/// Issuance monitoring: per-day credential/resolve volume split by key type
/// plus ECDH migration rates (the Phase-3 sunset signal).
class StatsPage extends StatefulWidget {
  const StatsPage({super.key});

  @override
  State<StatsPage> createState() => _StatsPageState();
}

class _StatsPageState extends State<StatsPage> {
  int _days = 7;
  List<dynamic> _rows = [];
  bool _busy = true;
  String? _error;

  static const _options = [7, 30, 90];

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
      final data = await api.get(
          '/api/v1/admin/stats/issuance', {'days': '$_days'}) as Map<String, dynamic>;
      final rows = ((data['days'] as List?) ?? []).reversed.toList();
      setState(() => _rows = rows);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = M3ETheme.of(context);
    num sum(String k) =>
        _rows.fold<num>(0, (a, r) => a + ((r as Map)[k] as num? ?? 0));
    final cred = sum('credentialTotal');
    final credEcdh = sum('credentialEcdh');
    final res = sum('resolveTotal');
    final resEcdh = sum('resolveEcdh');
    final failed = sum('resolveFailed');

    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('迁入监控（近$_days天）',
                  style: theme.typography.emphasized.headlineSmall),
              const Spacer(),
              M3EButtonGroup(
                type: M3EButtonGroupType.connected,
                selectedIndex: _options.indexOf(_days),
                onSelectedIndexChanged: (i) {
                  setState(
                      () => _days = _options[i ?? _options.indexOf(_days)]);
                  _load();
                },
                actions: const [
                  M3EButtonGroupAction(label: Text('7天')),
                  M3EButtonGroupAction(label: Text('30天')),
                  M3EButtonGroupAction(label: Text('90天')),
                ],
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text(
            '凭据 ECDH ${cred == 0 ? '-' : '${(100 * credEcdh / cred).toStringAsFixed(1)}%'}'
            '（$credEcdh/$cred） · '
            '解析 ECDH ${res == 0 ? '-' : '${(100 * resEcdh / res).toStringAsFixed(1)}%'}'
            '（$resEcdh/$res） · 失败兑换 $failed',
            style: theme.typography.emphasized.headlineSmall,
          ),
          const SizedBox(height: 12),
          Expanded(
            child: _busy
                ? const Center(child: M3ELoadingIndicator())
                : _error != null
                    ? Center(child: Text(_error!))
                    : M3EList(
                        itemCount: _rows.length,
                        itemBuilder: (context, i) {
                          final r = _rows[i] as Map<String, dynamic>;
                          final rate =
                              ((r['resolveEcdhRate'] as num?) ?? 0).toDouble();
                          return M3EListItem(
                            headline: '${r['date']}',
                            supportingText:
                                '凭据 ${r['credentialTotal']}（ECDH ${r['credentialEcdh']}）· '
                                '解析 ${r['resolveTotal']}（ECDH ${r['resolveEcdh']}）· '
                                '失败 ${r['resolveFailed']}',
                            trailing: SizedBox(
                              width: 120,
                              child: M3EProgressIndicator.linearWavy(
                                  value: rate.clamp(0.0, 1.0)),
                            ),
                          );
                        },
                      ),
          ),
        ],
      ),
    );
  }
}
