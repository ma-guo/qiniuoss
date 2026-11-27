package main

import (
	"fmt"
	"strings"
)

// langInfo is a language the app can show, with its tag and native name.
type langInfo struct {
	Tag  string
	Name string
}

// appLanguages are the languages the app supports. The first is the
// source language (the messages are written in it).
var appLanguages = []langInfo{
	{"zh", "中文"},
	{"en", "English"},
	{"ja", "日本語"},
	{"ko", "한국어"},
	{"de", "Deutsch"},
	{"fr", "Français"},
	{"pt", "Português"},
	{"ru", "Русский"},
	{"pl", "Polski"},
	{"fa", "فارسی"},
	{"ar", "العربية"},
}

// normalizeLang maps a BCP 47 tag to one of appLanguages, or "en" when
// the language is not supported.
func normalizeLang(tag string) string {
	t := strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(t, "-_"); i >= 0 {
		t = t[:i]
	}
	for _, l := range appLanguages {
		if l.Tag == t {
			return t
		}
	}
	return "en"
}

func langName(tag string) string {
	for _, l := range appLanguages {
		if l.Tag == tag {
			return l.Name
		}
	}
	return tag
}

// t translates a source (Chinese) message into the current language. A
// message with no translation is shown as written.
func (s *appState) t(zh string) string { return translate(s.lang.Get(), zh) }

// translate is t without the app state, for tests.
func translate(lang, zh string) string {
	if lang == "" || lang == "zh" {
		return zh
	}
	if m := catalog[lang]; m != nil {
		if v, ok := m[zh]; ok {
			return v
		}
	}
	return zh
}

// tf translates then formats; the translations keep the fmt verbs.
func (s *appState) tf(zh string, args ...any) string {
	return fmt.Sprintf(s.t(zh), args...)
}

// translation is one message in every language but the source.
type translation struct {
	zh, en, ja, ko, de, fr, pt, ru, pl, fa, ar string
}

// catalog is lang → source message → translation, built from
// translations once at start.
var catalog = buildCatalog()

func buildCatalog() map[string]map[string]string {
	targets := []string{"en", "ja", "ko", "de", "fr", "pt", "ru", "pl", "fa", "ar"}
	m := make(map[string]map[string]string, len(targets))
	for _, tag := range targets {
		m[tag] = make(map[string]string, len(translations))
	}
	for _, e := range translations {
		vals := []string{e.en, e.ja, e.ko, e.de, e.fr, e.pt, e.ru, e.pl, e.fa, e.ar}
		for i, tag := range targets {
			m[tag][e.zh] = vals[i]
		}
	}
	return m
}

// translations is the message catalog. The first field is the source
// message as written in the code.
var translations = []translation{
	{"七牛文件管理", "Qiniu File Manager", "七牛ファイルマネージャー", "치누 파일 관리자", "Qiniu Dateimanager", "Gestionnaire de fichiers Qiniu", "Gerenciador de Arquivos Qiniu", "Менеджер файлов Qiniu", "Menedżer plików Qiniu", "مدیر فایل کیونو", "مدير ملفات Qiniu"},
	{"浏览", "Browse", "閲覧", "탐색", "Durchsuchen", "Parcourir", "Navegar", "Обзор", "Przeglądaj", "مرور", "تصفح"},
	{"高级搜索", "Advanced Search", "詳細検索", "고급 검색", "Erweiterte Suche", "Recherche avancée", "Pesquisa avançada", "Расширенный поиск", "Wyszukiwanie zaawansowane", "جستجوی پیشرفته", "بحث متقدم"},
	{"账户", "Account", "アカウント", "계정", "Konto", "Compte", "Conta", "Аккаунт", "Konto", "حساب", "الحساب"},
	{"刷新", "Refresh", "更新", "새로 고침", "Aktualisieren", "Actualiser", "Atualizar", "Обновить", "Odśwież", "بازخوانی", "تحديث"},
	{"存储空间", "Buckets", "バケット", "버킷", "Speicherplätze", "Espaces", "Espaços", "Хранилища", "Przestrzenie", "فضاهای ذخیره‌سازی", "المساحات"},
	{"上传", "Upload", "アップロード", "업로드", "Hochladen", "Téléverser", "Enviar", "Загрузить", "Wgraj", "بارگذاری", "رفع"},
	{"上传列表", "Uploads", "アップロード一覧", "업로드 목록", "Upload-Liste", "Liste des envois", "Lista de envios", "Список загрузок", "Lista wysyłania", "فهرست بارگذاری", "قائمة الرفع"},
	{"上一级", "Up", "上へ", "상위", "Nach oben", "Niveau supérieur", "Acima", "Вверх", "W górę", "بالا", "أعلى"},
	{"过滤当前目录…", "Filter this folder…", "このフォルダを絞り込み…", "현재 폴더 필터…", "Ordner filtern…", "Filtrer ce dossier…", "Filtrar esta pasta…", "Фильтр папки…", "Filtruj folder…", "فیلتر پوشه…", "تصفية المجلد…"},
	{"新建", "New", "新規", "새로 만들기", "Neu", "Nouveau", "Novo", "Создать", "Nowy", "جدید", "جديد"},
	{"域名", "Domain", "ドメイン", "도메인", "Domain", "Domaine", "Domínio", "Домен", "Domena", "دامنه", "النطاق"},
	{"菜单", "Menu", "メニュー", "메뉴", "Menü", "Menu", "Menu", "Меню", "Menu", "منو", "القائمة"},
	{"未选择任何项", "Nothing selected", "未選択", "선택 없음", "Nichts ausgewählt", "Aucune sélection", "Nada selecionado", "Ничего не выбрано", "Nic nie wybrano", "چیزی انتخاب نشده", "لا يوجد تحديد"},
	{"全选", "Select all", "すべて選択", "전체 선택", "Alle auswählen", "Tout sélectionner", "Selecionar tudo", "Выбрать все", "Zaznacz wszystko", "انتخاب همه", "تحديد الكل"},
	{"已勾选 %d 项", "%d selected", "%d 件選択", "%d개 선택", "%d ausgewählt", "%d sélectionné(s)", "%d selecionado(s)", "Выбрано: %d", "Wybrano %d", "%d انتخاب‌شده", "تم تحديد %d"},
	{"当前：%s", "Current: %s", "現在: %s", "현재: %s", "Aktuell: %s", "Actuel : %s", "Atual: %s", "Текущий: %s", "Bieżący: %s", "فعلی: %s", "الحالي: %s"},
	{"预览", "Preview", "プレビュー", "미리보기", "Vorschau", "Aperçu", "Pré-visualizar", "Просмотр", "Podgląd", "پیش‌نمایش", "معاينة"},
	{"下载", "Download", "ダウンロード", "다운로드", "Herunterladen", "Télécharger", "Baixar", "Скачать", "Pobierz", "دانلود", "تنزيل"},
	{"复制到", "Copy to", "コピー先", "복사", "Kopieren nach", "Copier vers", "Copiar para", "Копировать в", "Kopiuj do", "کپی به", "نسخ إلى"},
	{"移动到", "Move to", "移動先", "이동", "Verschieben nach", "Déplacer vers", "Mover para", "Переместить в", "Przenieś do", "انتقال به", "نقل إلى"},
	{"复制链接", "Copy link", "リンクをコピー", "링크 복사", "Link kopieren", "Copier le lien", "Copiar link", "Копировать ссылку", "Kopiuj link", "کپی پیوند", "نسخ الرابط"},
	{"删除", "Delete", "削除", "삭제", "Löschen", "Supprimer", "Excluir", "Удалить", "Usuń", "حذف", "حذف"},
	{"清除选择", "Clear selection", "選択を解除", "선택 해제", "Auswahl aufheben", "Effacer la sélection", "Limpar seleção", "Снять выделение", "Wyczyść wybór", "پاک‌کردن انتخاب", "مسح التحديد"},
	{"名称", "Name", "名前", "이름", "Name", "Nom", "Nome", "Имя", "Nazwa", "نام", "الاسم"},
	{"大小", "Size", "サイズ", "크기", "Größe", "Taille", "Tamanho", "Размер", "Rozmiar", "اندازه", "الحجم"},
	{"类型", "Type", "種類", "유형", "Typ", "Type", "Tipo", "Тип", "Typ", "نوع", "النوع"},
	{"修改时间", "Modified", "更新日時", "수정 시간", "Geändert", "Modifié", "Modificado", "Изменён", "Zmodyfikowano", "تغییر", "تاريخ التعديل"},
	{"此目录为空", "This folder is empty", "このフォルダは空です", "이 폴더는 비어 있습니다", "Dieser Ordner ist leer", "Ce dossier est vide", "Esta pasta está vazia", "Папка пуста", "Ten folder jest pusty", "این پوشه خالی است", "هذا المجلد فارغ"},
	{"加载中…", "Loading…", "読み込み中…", "불러오는 중…", "Wird geladen…", "Chargement…", "Carregando…", "Загрузка…", "Ładowanie…", "در حال بارگذاری…", "جارٍ التحميل…"},
	{"加载更多", "Load more", "さらに読み込む", "더 보기", "Mehr laden", "Charger plus", "Carregar mais", "Загрузить ещё", "Wczytaj więcej", "بارگذاری بیشتر", "تحميل المزيد"},
	{"尚未配置账户，请点击左侧「账户」添加 AccessKey / SecretKey", "No account yet. Click \u201cAccount\u201d on the left to add an AccessKey / SecretKey.", "アカウントがありません。左の「アカウント」から AccessKey / SecretKey を追加してください。", "계정이 없습니다. 왼쪽 “계정”에서 AccessKey / SecretKey를 추가하세요.", "Noch kein Konto. Klicken Sie links auf „Konto“, um AccessKey / SecretKey hinzuzufügen.", "Aucun compte. Cliquez sur « Compte » à gauche pour ajouter une AccessKey / SecretKey.", "Nenhuma conta. Clique em \u201cConta\u201d à esquerda para adicionar AccessKey / SecretKey.", "Нет аккаунта. Нажмите «Аккаунт» слева, чтобы добавить AccessKey / SecretKey.", "Brak konta. Kliknij „Konto” po lewej, aby dodać AccessKey / SecretKey.", "هنوز حسابی نیست. برای افزودن AccessKey / SecretKey روی «حساب» در سمت چپ بزنید.", "لا يوجد حساب. انقر على «الحساب» على اليسار لإضافة AccessKey / SecretKey."},
	{"请选择左侧的存储空间", "Choose a bucket on the left", "左のバケットを選択してください", "왼쪽에서 버킷을 선택하세요", "Wählen Sie links einen Speicherplatz", "Choisissez un espace à gauche", "Escolha um espaço à esquerda", "Выберите хранилище слева", "Wybierz przestrzeń po lewej", "یک فضای ذخیره‌سازی در سمت چپ انتخاب کنید", "اختر مساحة على اليسار"},
	{"打开", "Open", "開く", "열기", "Öffnen", "Ouvrir", "Abrir", "Открыть", "Otwórz", "باز کردن", "فتح"},
	{"详情", "Details", "詳細", "세부 정보", "Details", "Détails", "Detalhes", "Детали", "Szczegóły", "جزئیات", "التفاصيل"},
	{"重命名", "Rename", "名前を変更", "이름 바꾸기", "Umbenennen", "Renommer", "Renomear", "Переименовать", "Zmień nazwę", "تغییر نام", "إعادة تسمية"},
	{"复制到…", "Copy to…", "コピー先…", "복사…", "Kopieren nach…", "Copier vers…", "Copiar para…", "Копировать в…", "Kopiuj do…", "کپی به…", "نسخ إلى…"},
	{"移动到…", "Move to…", "移動先…", "이동…", "Verschieben nach…", "Déplacer vers…", "Mover para…", "Переместить в…", "Przenieś do…", "انتقال به…", "نقل إلى…"},
	{"在浏览器打开", "Open in browser", "ブラウザで開く", "브라우저에서 열기", "Im Browser öffnen", "Ouvrir dans le navigateur", "Abrir no navegador", "Открыть в браузере", "Otwórz w przeglądarce", "باز کردن در مرورگر", "فتح في المتصفح"},
	{"更多操作", "More actions", "その他の操作", "더 많은 작업", "Weitere Aktionen", "Plus d'actions", "Mais ações", "Ещё действия", "Więcej akcji", "اقدامات بیشتر", "إجراءات أخرى"},
	{"新建文件夹", "New folder", "新しいフォルダ", "새 폴더", "Neuer Ordner", "Nouveau dossier", "Nova pasta", "Новая папка", "Nowy folder", "پوشه جدید", "مجلد جديد"},
	{"文件夹名称", "Folder name", "フォルダ名", "폴더 이름", "Ordnername", "Nom du dossier", "Nome da pasta", "Имя папки", "Nazwa folderu", "نام پوشه", "اسم المجلد"},
	{"新名称", "New name", "新しい名前", "새 이름", "Neuer Name", "Nouveau nom", "Novo nome", "Новое имя", "Nowa nazwa", "نام جدید", "اسم جديد"},
	{"确定", "OK", "OK", "확인", "OK", "OK", "OK", "ОК", "OK", "تأیید", "موافق"},
	{"取消", "Cancel", "キャンセル", "취소", "Abbrechen", "Annuler", "Cancelar", "Отмена", "Anuluj", "لغو", "إلغاء"},
	{"关闭", "Close", "閉じる", "닫기", "Schließen", "Fermer", "Fechar", "Закрыть", "Zamknij", "بستن", "إغلاق"},
	{"保存", "Save", "保存", "저장", "Speichern", "Enregistrer", "Salvar", "Сохранить", "Zapisz", "ذخیره", "حفظ"},
	{"账户管理", "Accounts", "アカウント管理", "계정 관리", "Konten", "Comptes", "Contas", "Аккаунты", "Konta", "مدیریت حساب‌ها", "إدارة الحسابات"},
	{"已有账户", "Existing accounts", "既存のアカウント", "기존 계정", "Vorhandene Konten", "Comptes existants", "Contas existentes", "Существующие аккаунты", "Istniejące konta", "حساب‌های موجود", "الحسابات الحالية"},
	{"使用", "Use", "使用", "사용", "Verwenden", "Utiliser", "Usar", "Использовать", "Użyj", "استفاده", "استخدام"},
	{"从 qshell 导入", "Import from qshell", "qshell から取り込み", "qshell에서 가져오기", "Aus qshell importieren", "Importer depuis qshell", "Importar do qshell", "Импорт из qshell", "Importuj z qshell", "ورود از qshell", "استيراد من qshell"},
	{"配置七牛 AccessKey / SecretKey，密钥仅保存在本机。", "Configure the Qiniu AccessKey / SecretKey. The keys are stored on this device only.", "七牛の AccessKey / SecretKey を設定します。キーはこの端末にのみ保存されます。", "Qiniu AccessKey / SecretKey를 설정합니다. 키는 이 기기에만 저장됩니다.", "Qiniu AccessKey / SecretKey konfigurieren. Die Schlüssel werden nur auf diesem Gerät gespeichert.", "Configurez l'AccessKey / SecretKey Qiniu. Les clés ne sont enregistrées que sur cet appareil.", "Configure a AccessKey / SecretKey do Qiniu. As chaves são salvas apenas neste dispositivo.", "Укажите AccessKey / SecretKey Qiniu. Ключи хранятся только на этом устройстве.", "Skonfiguruj AccessKey / SecretKey Qiniu. Klucze są przechowywane tylko na tym urządzeniu.", "AccessKey / SecretKey کیونو را تنظیم کنید. کلیدها فقط روی این دستگاه ذخیره می‌شوند.", "اضبط AccessKey / SecretKey الخاص بـ Qiniu. تُحفظ المفاتيح على هذا الجهاز فقط."},
	{"S3 密钥（可选，用于没有绑定域名时下载/预览）", "S3 keys (optional, to download/preview without a bound domain)", "S3 キー（任意、ドメイン未設定時のダウンロード/プレビュー用）", "S3 키(선택, 도메인 없이 다운로드/미리보기용)", "S3-Schlüssel (optional, zum Herunterladen/Ansehen ohne gebundene Domain)", "Clés S3 (facultatif, pour télécharger/apercevoir sans domaine)", "Chaves S3 (opcional, para baixar/pré-visualizar sem domínio)", "Ключи S3 (необязательно, для скачивания/просмотра без домена)", "Klucze S3 (opcjonalne, do pobierania/podglądu bez domeny)", "کلیدهای S3 (اختیاری، برای دانلود/پیش‌نمایش بدون دامنه)", "مفاتيح S3 (اختياري، للتنزيل/المعاينة بدون نطاق)"},
	{"控制台创建的 S3 AccessKey", "S3 AccessKey created in the console", "コンソールで作成した S3 AccessKey", "콘솔에서 생성한 S3 AccessKey", "Im Konsol erstellter S3 AccessKey", "S3 AccessKey créé dans la console", "S3 AccessKey criado no console", "S3 AccessKey, созданный в консоли", "S3 AccessKey utworzony w konsoli", "S3 AccessKey ساخته‌شده در کنسول", "S3 AccessKey مُنشأ في وحدة التحكم"},
	{"留空时使用上面的 AccessKey / SecretKey 进行 S3 签名。", "When empty, the AccessKey / SecretKey above are used to sign S3.", "空欄の場合は上の AccessKey / SecretKey で S3 署名します。", "비워 두면 위의 AccessKey / SecretKey로 S3에 서명합니다.", "Leer: Der AccessKey / SecretKey oben wird für die S3-Signatur verwendet.", "Vide : l'AccessKey / SecretKey ci-dessus sert à signer le S3.", "Vazio: o AccessKey / SecretKey acima é usado para assinar o S3.", "Пусто: для подписи S3 используется AccessKey / SecretKey выше.", "Puste: do podpisu S3 użyty będzie AccessKey / SecretKey powyżej.", "خالی: از AccessKey / SecretKey بالا برای امضای S3 استفاده می‌شود.", "عند تركه فارغاً يُستخدم AccessKey / SecretKey أعلاه لتوقيع S3."},
	{"例如：生产环境", "e.g. Production", "例: 本番環境", "예: 프로덕션", "z. B. Produktion", "ex. : Production", "ex.: Produção", "например: Продакшн", "np. Produkcja", "مثلاً: محیط تولید", "مثال: الإنتاج"},
	{"从 qshell 读取账户信息，请保存", "Read the qshell account; click Save", "qshell のアカウントを読み取りました。保存してください", "qshell 계정을 읽었습니다. 저장하세요", "qshell-Konto gelesen; bitte speichern", "Compte qshell lu ; enregistrez", "Conta do qshell lida; salve", "Аккаунт qshell прочитан; сохраните", "Odczytano konto qshell; zapisz", "حساب qshell خوانده شد؛ ذخیره کنید", "تمت قراءة حساب qshell؛ احفظه"},
	{"文件详情", "File details", "ファイル詳細", "파일 세부 정보", "Dateidetails", "Détails du fichier", "Detalhes do arquivo", "Сведения о файле", "Szczegóły pliku", "جزئیات فایل", "تفاصيل الملف"},
	{"上传时间", "Uploaded", "アップロード日時", "업로드 시간", "Hochgeladen", "Téléversé", "Enviado", "Загружено", "Wysłano", "زمان بارگذاری", "تاريخ الرفع"},
	{"确认删除", "Confirm delete", "削除の確認", "삭제 확인", "Löschen bestätigen", "Confirmer la suppression", "Confirmar exclusão", "Подтвердите удаление", "Potwierdź usunięcie", "تأیید حذف", "تأكيد الحذف"},
	{"将删除 %d 项（目录会连同其中所有对象一并删除），此操作不可撤销。", "Delete %d item(s) (folders delete all objects inside). This cannot be undone.", "%d 件を削除します（フォルダは中の全オブジェクトも削除）。元に戻せません。", "%d개를 삭제합니다(폴더는 내부 객체도 모두 삭제). 되돌릴 수 없습니다.", "%d Element(e) löschen (Ordner löschen alle Objekte darin). Nicht rückgängig zu machen.", "Supprimer %d élément(s) (les dossiers suppriment tout leur contenu). Irréversible.", "Excluir %d item(ns) (pastas excluem todo o conteúdo). Irreversível.", "Удалить %d элемент(ов) (папки — со всем содержимым). Необратимо.", "Usunąć %d pozycji (foldery wraz z zawartością). Nieodwracalne.", "حذف %d مورد (پوشه‌ها همراه با محتوا). بازگشت‌ناپذیر.", "حذف %d عنصر (المجلدات تحذف كل محتواها). لا يمكن التراجع."},
	{"目标存储空间", "Destination bucket", "コピー先バケット", "대상 버킷", "Ziel-Speicherplatz", "Espace de destination", "Espaço de destino", "Целевое хранилище", "Docelowa przestrzeń", "فضای مقصد", "المساحة الهدف"},
	{"目标前缀（目录，可手动输入）", "Destination prefix (folder, type or pick)", "コピー先プレフィックス（フォルダ、入力可）", "대상 접두사(폴더, 직접 입력 가능)", "Zielpräfix (Ordner, eingeben oder wählen)", "Préfixe de destination (dossier, saisir ou choisir)", "Prefixo de destino (pasta, digitar ou escolher)", "Целевой префикс (папка, ввод или выбор)", "Prefiks docelowy (folder, wpisz lub wybierz)", "پیشوند مقصد (پوشه، دستی یا انتخاب)", "بادئة الهدف (مجلد، إدخال أو اختيار)"},
	{"或从已有目录树中选择（点击「选择」）", "Or pick from the folder tree (click \u201cSelect\u201d)", "またはフォルダツリーから選択（「選択」をクリック）", "또는 폴더 트리에서 선택(“선택” 클릭)", "Oder aus dem Ordnerbaum wählen („Auswählen“)", "Ou choisissez dans l'arborescence (« Sélectionner »)", "Ou escolha na árvore de pastas (\u201cSelecionar\u201d)", "Или выберите в дереве папок («Выбрать»)", "Lub wybierz z drzewa folderów („Wybierz”)", "یا از درخت پوشه انتخاب کنید («انتخاب»)", "أو اختر من شجرة المجلدات («اختيار»)"},
	{"选择", "Select", "選択", "선택", "Auswählen", "Sélectionner", "Selecionar", "Выбрать", "Wybierz", "انتخاب", "اختيار"},
	{"目录树", "Folder tree", "フォルダツリー", "폴더 트리", "Ordnerbaum", "Arborescence", "Árvore de pastas", "Дерево папок", "Drzewo folderów", "درخت پوشه", "شجرة المجلدات"},
	{"（没有子目录，可直接在下方输入前缀）", "(No subfolders; type the prefix below)", "（サブフォルダなし。下にプレフィックスを入力）", "(하위 폴더 없음. 아래에 접두사를 입력하세요)", "(Keine Unterordner; Präfix unten eingeben)", "(Aucun sous-dossier ; saisissez le préfixe ci-dessous)", "(Sem subpastas; digite o prefixo abaixo)", "(Нет подпапок; введите префикс ниже)", "(Brak podfolderów; wpisz prefiks poniżej)", "(زیرپوشه‌ای نیست؛ پیشوند را پایین وارد کنید)", "(لا توجد مجلدات فرعية؛ أدخل البادئة أدناه)"},
	{"高级搜索（对齐 qshell listbucket2）", "Advanced search (like qshell listbucket2)", "詳細検索（qshell listbucket2 相当）", "고급 검색(qshell listbucket2와 동일)", "Erweiterte Suche (wie qshell listbucket2)", "Recherche avancée (comme qshell listbucket2)", "Pesquisa avançada (como qshell listbucket2)", "Расширенный поиск (как qshell listbucket2)", "Wyszukiwanie zaawansowane (jak qshell listbucket2)", "جستجوی پیشرفته (مانند qshell listbucket2)", "بحث متقدم (مثل qshell listbucket2)"},
	{"前缀 Prefix", "Prefix", "プレフィックス", "접두사", "Präfix", "Préfixe", "Prefixo", "Префикс", "Prefiks", "پیشوند", "البادئة"},
	{"后缀 Suffixes", "Suffixes", "サフィックス", "접미사", "Suffixe", "Suffixes", "Sufixos", "Суффиксы", "Przyrostki", "پسوندها", "اللواحق"},
	{"MIME 类型", "MIME type", "MIME タイプ", "MIME 유형", "MIME-Typ", "Type MIME", "Tipo MIME", "MIME-тип", "Typ MIME", "نوع MIME", "نوع MIME"},
	{"存储类型", "Storage class", "ストレージクラス", "스토리지 클래스", "Speicherklasse", "Classe de stockage", "Classe de armazenamento", "Класс хранения", "Klasa przechowywania", "کلاس ذخیره‌سازی", "فئة التخزين"},
	{"最小大小", "Min size", "最小サイズ", "최소 크기", "Min. Größe", "Taille min.", "Tamanho mín.", "Мин. размер", "Min. rozmiar", "حداقل اندازه", "أدنى حجم"},
	{"最大大小", "Max size", "最大サイズ", "최대 크기", "Max. Größe", "Taille max.", "Tamanho máx.", "Макс. размер", "Maks. rozmiar", "حداکثر اندازه", "أقصى حجم"},
	{"起始时间", "Start time", "開始時刻", "시작 시간", "Startzeit", "Heure de début", "Hora inicial", "Начало", "Czas początkowy", "زمان شروع", "وقت البداية"},
	{"结束时间", "End time", "終了時刻", "종료 시간", "Endzeit", "Heure de fin", "Hora final", "Окончание", "Czas końcowy", "زمان پایان", "وقت النهاية"},
	{"包含目录", "Include folders", "フォルダを含む", "폴더 포함", "Ordner einbeziehen", "Inclure les dossiers", "Incluir pastas", "Включая папки", "Uwzględnij foldery", "شامل پوشه‌ها", "تضمين المجلدات"},
	{"可读大小", "Human-readable size", "読みやすいサイズ", "가독성 크기", "Lesbare Größe", "Taille lisible", "Tamanho legível", "Читаемый размер", "Czytelny rozmiar", "اندازه خوانا", "حجم مقروء"},
	{"导出含表头", "Export with header", "ヘッダー付きでエクスポート", "헤더 포함 내보내기", "Mit Kopfzeile exportieren", "Exporter avec en-tête", "Exportar com cabeçalho", "Экспорт с заголовком", "Eksportuj z nagłówkiem", "خروجی با سرصفحه", "تصدير مع العنوان"},
	{"制表符分隔", "Tab-separated", "タブ区切り", "탭 구분", "Tabulatorgetrennt", "Séparé par tabulation", "Separado por tabulação", "Через табуляцию", "Tabulatory", "جدا شده با تب", "مفصول بعلامة جدولة"},
	{"逗号分隔", "Comma-separated", "カンマ区切り", "쉼표 구분", "Kommagetrennt", "Séparé par virgule", "Separado por vírgula", "Через запятую", "Przecinki", "جدا شده با کاما", "مفصول بفاصلة"},
	{"开始搜索", "Search", "検索", "검색", "Suchen", "Rechercher", "Pesquisar", "Поиск", "Szukaj", "جستجو", "بحث"},
	{"停止", "Stop", "停止", "중지", "Stopp", "Arrêter", "Parar", "Стоп", "Zatrzymaj", "توقف", "إيقاف"},
	{"导出结果", "Export results", "結果をエクスポート", "결과 내보내기", "Ergebnisse exportieren", "Exporter les résultats", "Exportar resultados", "Экспорт результатов", "Eksportuj wyniki", "خروجی نتایج", "تصدير النتائج"},
	{"已扫描 %d 项，匹配 %d 项", "Scanned %d, matched %d", "%d 件を走査、%d 件一致", "%d개 검사, %d개 일치", "%d geprüft, %d Treffer", "%d analysés, %d correspondants", "%d verificados, %d correspondentes", "Проверено %d, совпало %d", "Sprawdzono %d, dopasowano %d", "%d بررسی، %d منطبق", "تم فحص %d، تطابق %d"},
	{"设置条件后点击“开始搜索”", "Set filters, then click \u201cSearch\u201d", "条件を設定し「検索」をクリック", "조건 설정 후 “검색” 클릭", "Filter setzen, dann „Suchen“", "Définissez les filtres puis « Rechercher »", "Defina os filtros e clique em \u201cPesquisar\u201d", "Задайте фильтры и нажмите «Поиск»", "Ustaw filtry i kliknij „Szukaj”", "فیلترها را تنظیم و «جستجو» را بزنید", "عيّن عوامل التصفية ثم انقر «بحث»"},
	{"扫描中…", "Scanning…", "走査中…", "검사 중…", "Prüfe…", "Analyse…", "Verificando…", "Проверка…", "Sprawdzanie…", "در حال بررسی…", "جارٍ الفحص…"},
	{"没有可导出的结果", "No results to export", "エクスポートできる結果がありません", "내보낼 결과가 없습니다", "Keine Ergebnisse zum Export", "Aucun résultat à exporter", "Nenhum resultado para exportar", "Нет результатов для экспорта", "Brak wyników do eksportu", "نتیجه‌ای برای خروجی نیست", "لا توجد نتائج للتصدير"},
	{"全部类型", "All classes", "すべてのクラス", "모든 클래스", "Alle Klassen", "Toutes les classes", "Todas as classes", "Все классы", "Wszystkie klasy", "همه کلاس‌ها", "كل الفئات"},
	{"全部类型（清除）", "All classes (clear)", "すべてのクラス（解除）", "모든 클래스(해제)", "Alle Klassen (zurücksetzen)", "Toutes les classes (effacer)", "Todas as classes (limpar)", "Все классы (сбросить)", "Wszystkie klasy (wyczyść)", "همه کلاس‌ها (پاک‌سازی)", "كل الفئات (مسح)"},
	{"标准", "Standard", "標準", "표준", "Standard", "Standard", "Padrão", "Стандартный", "Standard", "استاندارد", "قياسي"},
	{"低频", "Infrequent", "低頻度", "저빈도", "Selten", "Peu fréquent", "Pouco frequente", "Низкочастотный", "Rzadki", "کم‌تکرار", "منخفض التكرار"},
	{"归档", "Archive", "アーカイブ", "아카이브", "Archiv", "Archive", "Arquivo", "Архивный", "Archiwum", "بایگانی", "أرشيف"},
	{"深度归档", "Deep archive", "ディープアーカイブ", "딥 아카이브", "Tiefes Archiv", "Archive profond", "Arquivo profundo", "Глубокий архив", "Głęboki archiwum", "بایگانی عمیق", "أرشيف عميق"},
	{"归档直读", "Archive (direct)", "アーカイブ直接", "아카이브 직접", "Direktes Archiv", "Archive direct", "Arquivo direto", "Прямой архив", "Archiwum bezpośrednie", "بایگانی مستقیم", "أرشيف مباشر"},
	{"清除已完成", "Clear finished", "完了を消去", "완료 지우기", "Abgeschlossene entfernen", "Effacer terminés", "Limpar concluídos", "Очистить завершённые", "Wyczyść ukończone", "پاک‌کردن تکمیل‌شده‌ها", "مسح المكتملة"},
	{"全部清除", "Clear all", "すべて消去", "모두 지우기", "Alle entfernen", "Tout effacer", "Limpar tudo", "Очистить всё", "Wyczyść wszystko", "پاک‌کردن همه", "مسح الكل"},
	{"收起", "Collapse", "折りたたむ", "접기", "Einklappen", "Réduire", "Recolher", "Свернуть", "Zwiń", "جمع‌کردن", "طيّ"},
	{"等待", "Waiting", "待機中", "대기 중", "Wartet", "En attente", "Aguardando", "Ожидание", "Oczekiwanie", "در انتظار", "بالانتظار"},
	{"上传中", "Uploading", "アップロード中", "업로드 중", "Wird hochgeladen", "Téléversement", "Enviando", "Загрузка", "Wysyłanie", "در حال بارگذاری", "جارٍ الرفع"},
	{"完成", "Done", "完了", "완료", "Fertig", "Terminé", "Concluído", "Готово", "Gotowe", "انجام شد", "تم"},
	{"失败", "Failed", "失敗", "실패", "Fehlgeschlagen", "Échec", "Falhou", "Ошибка", "Niepowodzenie", "ناموفق", "فشل"},
	{"语言", "Language", "言語", "언어", "Sprache", "Langue", "Idioma", "Язык", "Język", "زبان", "اللغة"},
	{"跟随系统", "System", "システムに従う", "시스템", "System", "Système", "Sistema", "Системный", "Systemowy", "سیستم", "النظام"},
	{"选择账户", "Select account", "アカウントを選択", "계정 선택", "Konto wählen", "Choisir un compte", "Selecionar conta", "Выбрать аккаунт", "Wybierz konto", "انتخاب حساب", "اختر الحساب"},
	{"账户 %d", "Account %d", "アカウント %d", "계정 %d", "Konto %d", "Compte %d", "Conta %d", "Аккаунт %d", "Konto %d", "حساب %d", "الحساب %d"},
	{"尚未配置账户，请点击左侧“管理账户”。", "No account yet. Click \u201cAccount\u201d on the left.", "アカウントがありません。左の「アカウント」をクリックしてください。", "계정이 없습니다. 왼쪽 “계정”을 클릭하세요.", "Noch kein Konto. Klicken Sie links auf „Konto“.", "Aucun compte. Cliquez sur « Compte » à gauche.", "Nenhuma conta. Clique em \u201cConta\u201d à esquerda.", "Нет аккаунта. Нажмите «Аккаунт» слева.", "Brak konta. Kliknij „Konto” po lewej.", "هنوز حسابی نیست. روی «حساب» در سمت چپ بزنید.", "لا يوجد حساب. انقر على «الحساب» على اليسار."},
}
