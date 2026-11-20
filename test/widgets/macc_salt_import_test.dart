import 'dart:io';
import 'dart:typed_data';

import 'package:cross_file/cross_file.dart';
import 'package:file_picker/file_picker.dart';
import 'package:fl_clash/views/tools.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import '../helpers/app_paths.dart';
import '../helpers/test_app.dart';

base class _PickedSalt extends PlatformFile {
  _PickedSalt(this._content);

  final String _content;

  @override
  String get name => 'macc.salt';

  @override
  Uri get uri => Uri.file('/macc.salt');

  @override
  XFile get xFile => XFile.fromData(Uint8List.fromList(_content.codeUnits));

  @override
  int lengthSync() => _content.length;

  @override
  Future<int> length() async => lengthSync();

  @override
  Future<Uint8List> readAsBytes() => xFile.readAsBytes();

  @override
  Stream<Uint8List> readAsByteStream() async* {
    yield await readAsBytes();
  }
}

void main() {
  testWidgets(
    'imports a MACC salt and preserves it on an invalid replacement',
    (tester) async {
      final directory = Directory.systemTemp.createTempSync('macc_salt_');
      addTearDown(() => directory.deleteSync(recursive: true));
      await useAppPaths(directory);
      final target = File('${directory.path}/macc.salt');
      var picked = _PickedSalt('test-salt\r\n');
      await tester.pumpWidget(
        TestApp(
          locale: const Locale('en'),
          child: MaccSaltImportItem(pickFile: () async => picked),
        ),
      );
      final row = find.text('Import MACC salt file');
      await tester.runAsync(() async {
        await tester.tap(row);
        await tester.pump();
        for (var attempt = 0; attempt < 50 && !target.existsSync(); attempt++) {
          await Future<void>.delayed(const Duration(milliseconds: 10));
        }
      });
      await tester.pumpAndSettle();
      expect(target.readAsStringSync(), 'test-salt');
      expect(find.textContaining('MACC salt saved'), findsOneWidget);
      await tester.runAsync(() async {
        await tester.tap(find.text('Confirm'));
        await tester.pumpAndSettle();
        picked = _PickedSalt('\n');
        await tester.tap(row);
      });
      await tester.pumpAndSettle();
      expect(find.textContaining('Choose a valid macc.salt'), findsOneWidget);
      expect(target.readAsStringSync(), 'test-salt');
    },
  );
}
