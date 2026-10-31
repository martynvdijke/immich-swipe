import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Persistent auth + reviewed-set storage backed by flutter_secure_storage.
class SecureStore {
  SecureStore({FlutterSecureStorage? storage})
      : _storage = storage ?? const FlutterSecureStorage();

  final FlutterSecureStorage _storage;

  static const _kServerUrl = 'serverUrl';
  static const _kLastServerUrl = 'lastServerUrl';
  static const _kToken = 'sessionToken';
  static const _kUserName = 'userName';
  static const _kMode = 'mode';
  static const _kReviewedPrefix = 'reviewed_';
  static const _kSkipVideos = 'skipVideos';

  Future<void> saveSession({
    required String serverUrl,
    required String token,
    required String userName,
    required String mode,
  }) async {
    await _storage.write(key: _kServerUrl, value: serverUrl);
    // Keep the last chosen backend around even after the session is cleared, so
    // the login screen can prefill it instead of the hosted default.
    await _storage.write(key: _kLastServerUrl, value: serverUrl);
    await _storage.write(key: _kToken, value: token);
    await _storage.write(key: _kUserName, value: userName);
    await _storage.write(key: _kMode, value: mode);
  }

  Future<Map<String, String?>> readSession() async {
    return {
      'serverUrl': await _storage.read(key: _kServerUrl),
      'token': await _storage.read(key: _kToken),
      'userName': await _storage.read(key: _kUserName),
      'mode': await _storage.read(key: _kMode),
    };
  }

  /// The backend URL from the most recent successful login, regardless of
  /// whether a session still exists. Falls back to the current session's URL so
  /// accounts saved before this preference existed keep their server. Returns
  /// null when nothing was saved.
  Future<String?> readLastServerUrl() async {
    try {
      final value = await _storage.read(key: _kLastServerUrl) ??
          await _storage.read(key: _kServerUrl);
      return value == null || value.trim().isEmpty ? null : value;
    } catch (_) {
      return null;
    }
  }

  Future<void> clearSession() async {
    await _storage.delete(key: _kServerUrl);
    await _storage.delete(key: _kToken);
    await _storage.delete(key: _kUserName);
    await _storage.delete(key: _kMode);
  }

  String _reviewedKey(String serverUrl, String userName) =>
      '$_kReviewedPrefix${serverUrl}_$userName';

  Future<Set<String>> readReviewed(String serverUrl, String userName) async {
    final v = await _storage.read(key: _reviewedKey(serverUrl, userName));
    if (v == null || v.isEmpty) return {};
    return v.split(',').where((e) => e.isNotEmpty).toSet();
  }

  Future<void> writeReviewed(
      String serverUrl, String userName, Set<String> ids) async {
    await _storage.write(key: _reviewedKey(serverUrl, userName), value: ids.join(','));
  }

  Future<void> addReviewed(String serverUrl, String userName, String id) async {
    final s = await readReviewed(serverUrl, userName);
    s.add(id);
    await writeReviewed(serverUrl, userName, s);
  }

  Future<bool> readSkipVideos() async {
    final v = await _storage.read(key: _kSkipVideos);
    return v == 'true';
  }

  Future<void> writeSkipVideos(bool v) async {
    await _storage.write(key: _kSkipVideos, value: v ? 'true' : 'false');
  }
}
