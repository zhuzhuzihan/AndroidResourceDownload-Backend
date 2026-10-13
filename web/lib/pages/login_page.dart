import 'package:material_3_expressive/material_3_expressive.dart';
import 'package:material_ui/material_ui.dart';

import '../api_client.dart';

/// Email-OTP admin login. GitHub OAuth ends at an Android deep link and QQ
/// needs the mobile client, so the console only offers email login.
class LoginPage extends StatefulWidget {
  final void Function(String access, String refresh) onAuthed;
  const LoginPage({super.key, required this.onAuthed});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _email = TextEditingController();
  final _code = TextEditingController();
  bool _sent = false;
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _email.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await api.post('/api/v1/auth/email/code', {'email': _email.text.trim()});
      setState(() => _sent = true);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _login() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final data = await api.post('/api/v1/auth/email/login', {
        'email': _email.text.trim(),
        'code': _code.text.trim(),
      }) as Map<String, dynamic>;
      final user = data['user'] as Map<String, dynamic>;
      if (user['role'] != 'ADMIN') {
        setState(() => _error = '该账号不是管理员');
        return;
      }
      widget.onAuthed(
        data['accessToken'] as String,
        data['refreshToken'] as String,
      );
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: M3EAppBar.top(titleText: 'ResDownload Admin'),
      body: Center(
        child: SizedBox(
          width: 360,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              M3ETextField(
                controller: _email,
                label: '管理员邮箱',
                showClearButton: true,
              ),
              const SizedBox(height: 12),
              if (_sent)
                M3ETextField(
                  controller: _code,
                  label: '验证码',
                ),
              if (_sent) const SizedBox(height: 12),
              if (_error != null)
                Text(_error!,
                    style: TextStyle(
                        color: M3ETheme.of(context).colorScheme.error)),
              if (_error != null) const SizedBox(height: 12),
              _busy
                  ? const M3ELoadingIndicator()
                  : Row(
                      mainAxisAlignment: MainAxisAlignment.end,
                      children: [
                        M3EButton.text(onPressed: _send, child: const Text('发送验证码')),
                        const SizedBox(width: 8),
                        M3EButton(
                          style: M3EButtonStyle.filled,
                          onPressed: _sent ? _login : null,
                          child: const Text('登录'),
                        ),
                      ],
                    ),
            ],
          ),
        ),
      ),
    );
  }
}
