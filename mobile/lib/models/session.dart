class SessionInfo {
  SessionInfo({required this.serverUrl, required this.token, required this.userName, required this.mode});
  final String serverUrl;
  final String token;
  final String userName;
  final String mode; // apiKey | accessToken
}

class LoginResult {
  LoginResult({required this.token, required this.userName, required this.serverUrl, required this.mode, required this.hasApiKey});
  final String token;
  final String userName;
  final String serverUrl;
  final String mode;
  final bool hasApiKey;

  factory LoginResult.fromJson(Map<String, dynamic> j) => LoginResult(
        token: j['token'] as String,
        userName: j['userName'] as String? ?? j['user'] as String? ?? '',
        serverUrl: j['serverUrl'] as String? ?? '',
        mode: j['mode'] as String? ?? 'apiKey',
        hasApiKey: j['hasApiKey'] as bool? ?? true,
      );
}

class AuthConfig {
  AuthConfig({required this.oauthEnabled, required this.oauthButtonText});
  final bool oauthEnabled;
  final String oauthButtonText;
  factory AuthConfig.fromJson(Map<String, dynamic> j) => AuthConfig(
        oauthEnabled: j['oauthEnabled'] as bool? ?? false,
        oauthButtonText: j['oauthButtonText'] as String? ?? 'SSO',
      );
}
