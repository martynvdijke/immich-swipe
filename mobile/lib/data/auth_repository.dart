import 'package:dio/dio.dart';
import '../core/api_client.dart';
import '../models/session.dart';

class AuthRepository {
  AuthRepository(this._dio);
  final Dio _dio;

  Future<AuthConfig> getConfig() async {
    try {
      final r = await _dio.get('/auth/config');
      return AuthConfig.fromJson(r.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<LoginResult> login({
    required String userName,
    required String password,
    String? serverUrl,
  }) async {
    try {
      final r = await _dio.post('/auth/login', data: {
        'userName': userName,
        'password': password,
        if (serverUrl != null && serverUrl.isNotEmpty) 'serverUrl': serverUrl,
      });
      return LoginResult.fromJson(r.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<LoginResult> createAccount({
    required String userName,
    required String password,
    String? apiKey,
    String? serverUrl,
  }) async {
    try {
      final r = await _dio.post('/auth/login', data: {
        'userName': userName,
        'password': password,
        'create': true,
        if (apiKey != null && apiKey.isNotEmpty) 'apiKey': apiKey,
        if (serverUrl != null && serverUrl.isNotEmpty) 'serverUrl': serverUrl,
      });
      return LoginResult.fromJson(r.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<void> setApiKey(String apiKey) async {
    try {
      await _dio.post('/auth/account/apikey', data: {'apiKey': apiKey});
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<Map<String, dynamic>> oauthStart({String? serverUrl}) async {
    try {
      final r = await _dio.post('/auth/oauth/start', data: {
        if (serverUrl != null && serverUrl.isNotEmpty) 'serverUrl': serverUrl,
      });
      return r.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<LoginResult> oauthFinish(String code) async {
    try {
      final r = await _dio.post('/auth/oauth/finish', data: {'code': code});
      return LoginResult.fromJson(r.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<void> logout() async {
    try {
      await _dio.post('/auth/logout');
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }
}
