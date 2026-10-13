import 'dart:convert';

import 'package:http/http.dart' as http;

import 'session.dart';

// Overridable at build time:
//   flutter build web --dart-define=API_BASE_URL=https://api.example.com
const apiBase = String.fromEnvironment(
  'API_BASE_URL',
  defaultValue: 'http://localhost:8080',
);

class ApiException implements Exception {
  final int code;
  final String message;
  ApiException(this.code, this.message);
  @override
  String toString() => 'ApiException($code): $message';
}

class ApiClient {
  Future<dynamic> _req(
    String method,
    String path, {
    Map<String, dynamic>? body,
    Map<String, String>? query,
  }) async {
    var uri = Uri.parse('$apiBase$path');
    if (query != null && query.isNotEmpty) {
      uri = uri.replace(queryParameters: query);
    }
    final headers = <String, String>{'Content-Type': 'application/json'};
    if (sessionToken != null) {
      headers['Authorization'] = 'Bearer $sessionToken';
    }
    late http.Response resp;
    final payload = body == null ? null : jsonEncode(body);
    switch (method) {
      case 'GET':
        resp = await http.get(uri, headers: headers);
        break;
      case 'POST':
        resp = await http.post(uri, headers: headers, body: payload);
        break;
      case 'DELETE':
        resp = await http.delete(uri, headers: headers);
        break;
      default:
        throw StateError('unsupported method $method');
    }
    final decoded = jsonDecode(resp.body) as Map<String, dynamic>;
    final code = decoded['code'] as int? ?? -1;
    if (code != 0) {
      if (code == 10001 || code == 10002) {
        sessionToken = null;
        onAuthLost?.call();
      }
      throw ApiException(code, decoded['message'] as String? ?? 'error');
    }
    return decoded['data'];
  }

  Future<dynamic> get(String path, [Map<String, String>? query]) =>
      _req('GET', path, query: query);

  Future<dynamic> post(String path, Map<String, dynamic> body) =>
      _req('POST', path, body: body);

  Future<dynamic> delete(String path) => _req('DELETE', path);
}

final api = ApiClient();
