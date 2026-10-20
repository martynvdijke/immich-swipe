import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  final manifest =
      File('android/app/src/main/AndroidManifest.xml').readAsStringSync();
  final gradle = File('android/app/build.gradle.kts').readAsStringSync();
  final networkSecurityConfig =
      File('android/app/src/main/res/xml/network_security_config.xml')
          .readAsStringSync();

  test('manifest grants INTERNET', () {
    expect(manifest, contains('android.permission.INTERNET'));
  });

  test('manifest permits self-hosted cleartext HTTP', () {
    expect(
      manifest,
      contains('android:networkSecurityConfig="@xml/network_security_config"'),
    );
    expect(networkSecurityConfig, contains('cleartextTrafficPermitted="true"'));
  });

  test('release build wires key.properties signing with a debug fallback', () {
    expect(gradle, contains('key.properties'));
    expect(gradle, contains('releaseSigningConfig'));
    expect(gradle, contains('signingConfigs.getByName("debug")'));
    expect(gradle, contains('create("release")'));
  });
}
