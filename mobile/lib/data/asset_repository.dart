import 'dart:typed_data';
import 'package:dio/dio.dart';
import '../core/api_client.dart';
import '../models/immich_asset.dart';

class AssetRepository {
  AssetRepository(this._dio);
  final Dio _dio;

  Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) async {
    try {
      final body = <String, dynamic>{'size': size, if (imageOnly) 'type': 'IMAGE'};
      final r = await _dio.post('/search/random', data: body);
      final list = (r.data as List).map((e) => ImmichAsset.fromJson(e as Map<String, dynamic>)).toList();
      return list;
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<MetadataSearchResponse> searchMetadata({
    int page = 1,
    int size = 50,
    String order = 'asc',
    String? assetType,
  }) async {
    try {
      final r = await _dio.post('/search/metadata', data: {
        'order': order,
        'page': page,
        'size': size,
        if (assetType != null) 'type': assetType, // ignore: use_null_aware_elements
      });
      return MetadataSearchResponse.fromJson(r.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<void> trash(String id) async {
    try {
      await _dio.delete('/assets', data: {'ids': [id], 'force': false});
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<void> restore(String id) async {
    try {
      await _dio.post('/trash/restore/assets', data: {'ids': [id]});
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<void> setFavorite(String id, bool fav) async {
    try {
      await _dio.put('/assets/$id', data: {'isFavorite': fav});
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async {
    try {
      final r = await _dio.get('/assets/$id/thumbnail',
          queryParameters: {'size': size},
          options: Options(responseType: ResponseType.bytes));
      return Uint8List.fromList(r.data as List<int>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<Uint8List> fetchOriginal(String id) async {
    try {
      final r = await _dio.get('/assets/$id/original',
          options: Options(responseType: ResponseType.bytes));
      return Uint8List.fromList(r.data as List<int>);
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }
}
