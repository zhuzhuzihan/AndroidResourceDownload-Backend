import 'package:material_3_expressive/material_3_expressive.dart';
import 'package:material_ui/material_ui.dart';

import 'api_client.dart';
import 'pages/alerts_page.dart';
import 'pages/login_page.dart';
import 'pages/logs_page.dart';
import 'pages/stats_page.dart';
import 'pages/versions_page.dart';
import 'session.dart';

void main() {
  runApp(const AdminApp());
}

class AdminApp extends StatefulWidget {
  const AdminApp({super.key});

  @override
  State<AdminApp> createState() => _AdminAppState();
}

class _AdminAppState extends State<AdminApp> {
  bool _loggedIn = false;

  @override
  void initState() {
    super.initState();
    onAuthLost = () {
      if (mounted && _loggedIn) setState(() => _loggedIn = false);
    };
  }

  @override
  Widget build(BuildContext context) {
    return M3EMaterialApp(
      title: 'ResDownload Admin',
      data: M3EThemeData.light(seedColor: const Color(0xFF6750A4)),
      autoTheming: true,
      dynamicColoring: true,
      home: _loggedIn
          ? AdminShell(onLogout: () async {
              try {
                final rt = sessionRefreshToken;
                if (rt != null) {
                  await api.post('/api/v1/auth/logout', {'refreshToken': rt});
                }
              } catch (_) {
                // Best effort; local logout always happens.
              }
              sessionToken = null;
              sessionRefreshToken = null;
              setState(() => _loggedIn = false);
            })
          : LoginPage(onAuthed: (access, refresh) {
              sessionToken = access;
              sessionRefreshToken = refresh;
              setState(() => _loggedIn = true);
            }),
    );
  }
}

class AdminShell extends StatefulWidget {
  final Future<void> Function() onLogout;
  const AdminShell({super.key, required this.onLogout});

  @override
  State<AdminShell> createState() => _AdminShellState();
}

class _AdminShellState extends State<AdminShell> {
  int _index = 0;

  static const _titles = ['版本发布', '迁入监控', '日志查询', '会话告警'];

  @override
  Widget build(BuildContext context) {
    final pages = [
      const VersionsPage(),
      const StatsPage(),
      const LogsPage(),
      const AlertsPage(),
    ];
    return Scaffold(
      appBar: M3EAppBar.top(
        titleText: 'ResDownload Admin · ${_titles[_index]}',
        actions: [
          M3EButton.text(
            onPressed: () {
              M3ETheme.controllerOf(context)?.toggleBrightness(
                fallback: M3ETheme.of(context).brightness,
                autoTheming: true,
              );
            },
            child: const Text('主题'),
          ),
          M3EButton.text(
            onPressed: () => widget.onLogout(),
            child: const Text('退出'),
          ),
        ],
      ),
      body: Row(
        children: [
          M3ENavigationRail(
            sections: const [
              M3ENavigationRailSection(
                destinations: [
                  M3ENavigationRailDestination(
                      icon: Icon(M3EIcons.inbox), label: '版本'),
                  M3ENavigationRailDestination(
                      icon: Icon(M3EIcons.schedule), label: '监控'),
                  M3ENavigationRailDestination(
                      icon: Icon(M3EIcons.search), label: '日志'),
                  M3ENavigationRailDestination(
                      icon: Icon(M3EIcons.mail), label: '告警'),
                ],
              ),
            ],
            selectedIndex: _index,
            onDestinationSelected: (i) => setState(() => _index = i),
          ),
          Expanded(child: pages[_index]),
        ],
      ),
    );
  }
}
