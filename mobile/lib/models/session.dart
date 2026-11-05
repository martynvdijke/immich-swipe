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
  AuthConfig({
    required this.oauthEnabled,
    required this.oauthButtonText,
    this.demoEnabled = false,
    this.demoUserName = '',
    this.demoPassword = '',
    this.demoServerUrl = '',
  });

  final bool oauthEnabled;
  final String oauthButtonText;

  /// Optional review/demo account published by the backend so app-store
  /// reviewers can try the app without their own Immich server. The credentials
  /// are intentionally public; the Immich API key never is.
  final bool demoEnabled;
  final String demoUserName;
  final String demoPassword;

  /// The Immich server the demo account proxies. This is informational only:
  /// the app's API base always stays the swipe backend URL the user entered.
  final String demoServerUrl;

  factory AuthConfig.fromJson(Map<String, dynamic> j) => AuthConfig(
        oauthEnabled: j['oauthEnabled'] as bool? ?? false,
        oauthButtonText: j['oauthButtonText'] as String? ?? 'SSO',
        demoEnabled: j['demoEnabled'] as bool? ?? false,
        demoUserName: j['demoUserName'] as String? ?? '',
        demoPassword: j['demoPassword'] as String? ?? '',
        demoServerUrl: j['demoServerUrl'] as String? ?? '',
      );
}
