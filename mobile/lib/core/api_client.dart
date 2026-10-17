import 'package:dio/dio.dart';

/// Dio client configured for the immich-swipe Go backend.
/// Base URL is `<serverUrl>/api` and every request carries
/// `Authorization: Bearer <swipe-session-token>`.
///
/// The Go proxy strips that header before forwarding and injects
/// Immich credentials (x-api-key or Bearer) server-side.
class ApiClient {
  ApiClient({required String serverUrl, required String? token})
      : _dio = Dio(BaseOptions(
          baseUrl: '${serverUrl.replaceAll(RegExp(r'/+$'), '')}/api',
          connectTimeout: const Duration(seconds: 15),
          receiveTimeout: const Duration(seconds: 30),
          headers: {'Accept': 'application/json'},
        )) {
    _dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) {
        if (token != null && token.isNotEmpty) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        // Ensure JSON content-type when body is present.
        if (options.data != null) {
          options.headers['Content-Type'] = 'application/json';
        }
        handler.next(options);
      },
    ));
  }

  final Dio _dio;
  Dio get dio => _dio;

  /// Normalise Dio error into a typed exception for repos to map.
  static AppException mapError(DioException e) {
    final status = e.response?.statusCode;
    final data = e.response?.data;
    String code = '';
    String message = e.message ?? 'Network error';
    if (data is Map) {
      code = (data['code'] ?? data['error'] ?? '').toString();
      message = (data['message'] ?? data['error'] ?? message).toString();
    } else if (data is String && data.isNotEmpty) {
      try {
        // Try to avoid showing raw HTML
        message = data.length > 500 ? data.substring(0, 500) : data;
      } catch (_) {}
    }
    if (status == 401) return AppException('unauthorized', message, statusCode: 401);
    if (status == 428) return AppException('api_key_required', message, statusCode: 428);
    if (code.isNotEmpty) return AppException(code, message, statusCode: status);
    return AppException('network_error', message, statusCode: status);
  }
}

class AppException implements Exception {
  AppException(this.code, this.message, {this.statusCode});
  final String code;
  final String message;
  final int? statusCode;
  @override
  String toString() => 'AppException($code): $message';
}
