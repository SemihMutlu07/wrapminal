# cc-lens vs. Agent Wrapped Karşılaştırma ve Analiz Raporu

Bu rapor, Claude Code odaklı **cc-lens** (Arindam200/cc-lens) projesi ile sizin çoklu ajan destekli **Agent Wrapped** projeniz arasındaki kavramsal ve teknik farkları incelemekte, projenin özgün yanlarını ortaya koymakta ve gelişim yol haritası için öneriler sunmaktadır.

---

## 1. Temel Farklar ve Konumlandırma

Sizin projenizin odak noktası, klasik bir yönetim/analitik aracı olmaktan ziyade **"AI Developer Wrapped"** (Spotify Wrapped benzeri, paylaşılabilir ve eğlenceli bir yıllık/dönemlik özet) konseptidir.

| Kriter | cc-lens (Arindam200) | Agent Wrapped (Sizin Projeniz) |
| :--- | :--- | :--- |
| **Ana Konsept** | SaaS-style / Linear benzeri sürekli takip dashboard'u. | Dönemsel/yıllık **"Wrapped"** özeti, gamification ve sosyal paylaşım. |
| **Kapsam (Ajan Desteği)** | Sadece **Claude Code** odaklı. | **11+ Ajan** (Claude Code, Codex, Gemini, Continue, Aider, Cursor, OpenCode, Cline, Windsurf, pi, hermes). |
| **Teknoloji Yığını** | Next.js, React, Tailwind CSS, TypeScript. | Go (Tek statik binary) + Vanilla JS/CSS (Sıfır bağımlılık). |
| **Görsel Kimlik** | Modern karanlık tema (SaaS dashboard). | Retro, neobrutalist krem rengi kağıt estetiği (Ekran görüntüsü için optimize). |
| **Gizlilik/Paylaşım Güvenliği** | Local çalışır ancak maskeleme yoktur. | **Varsayılan olarak maskeli** (Proje isimlerini stable kod adlarına çevirir). |
| **Çıktı Biçimleri** | JSON/JSONL export, data import. | **SVG formatında doğrudan Wrapped kartı dışa aktarma** (Sosyal medya dostu). |

---

## 2. İsim Değişikliği (Rebranding) Önerileri

Projenin Claude Code özelinde olmadığını vurgulamak ve "Wrapped" vizyonunu öne çıkarmak için alternatif isimler:

1. **`wrapped.dev` (veya `dev-wrapped`)**
   * *Neden:* Çok sade, akılda kalıcı ve doğrudan geliştirici dünyasındaki "Wrapped" akımına hitap ediyor.
2. **`unprompted`**
   * *Neden:* "Ajanların kendi kendine çalıştığı" bir dünyayı çağrıştıran, kelime oyunu içeren güçlü bir marka ismi.
3. **`agent-wrapped`**
   * *Neden:* Projenin en büyük gücü olan çoklu ajan (multi-agent) desteğini doğrudan isminde taşır.
4. **`stack-wrapped`**
   * *Neden:* Geliştiricinin kullandığı tüm yapay zeka araç yığınını (stack) kapsadığını belirtir.

---

## 3. cc-lens Projesinden Neler Öğrenebiliriz?

Mevcut projemizi neobrutalist/Wrapped çizgisinden sapmadan nasıl daha iyi hale getirebiliriz?

* **Model Bazlı Maliyet Hesaplama:** cc-lens'teki gibi yerel `pricing.json` mantığını entegre ederek, geliştiricinin o güne kadar hangi modelde ne kadar token harcadığını ve tahmini maliyetini/tasarrufunu Wrapped kartı olarak göstermek.
* **CLI Terminal Digest:** `cc-lens digest` gibi, browser açmaya gerek kalmadan doğrudan terminalde zengin bir ASCII/Tablo formatında özet sunan `--digest` parametresi.
* **SQLite Derinliği:** Cursor ve Cline/Roo gibi araçların yerel SQLite veritabanlarından daha detaylı tarihsel veri çekebilmek için sorguları optimize etmek.

---

## 4. Yol Haritası (Aksiyon Önerileri)

1. **Yeniden Adlandırma:** Yeni isim belirlendikten sonra tüm dosyalarda (`go.mod`, `README.md`, `static/index.html` vb.) isim güncellemelerinin yapılması.
2. **Maliyet/Token Kartı Entegrasyonu:** Wrapped ekranına model tabanlı maliyet tahmini yapan yeni bir kart eklenmesi.
3. **CLI Digest Parametresi:** Go tarafında terminalden hızlıca istatistik okumayı sağlayacak `--digest` özelliğinin geliştirilmesi.
