#!/usr/bin/env bash
# CLAUDE.md "Karar kuralları" §4 kapanış kontrolünü hatırlatır.
#
#   decisions-check.sh start  -> SessionStart: oturum için zaman damgası bırakır
#   decisions-check.sh stop   -> Stop: damgadan sonra dosya değiştiyse Claude'u
#                                kontrolü yapması için bir tur daha çalıştırır
#
# Değişiklik yoksa veya Claude zaten bu hook yüzünden devam ediyorsa sessiz çıkar.

set -euo pipefail

mode="${1:-stop}"
input="$(cat)"
root="${CLAUDE_PROJECT_DIR:-$(pwd)}"
state_dir="$root/.claude/.state"
session_id="$(jq -r '.session_id // "default"' <<<"$input")"
marker="$state_dir/decisions-$session_id"

mkdir -p "$state_dir"

if [[ "$mode" == "start" ]]; then
  touch "$marker"
  exit 0
fi

# Hook'un kendi tetiklediği devam turunda tekrar engelleme (sonsuz döngü koruması).
if [[ "$(jq -r '.stop_hook_active // false' <<<"$input")" == "true" ]]; then
  exit 0
fi

# SessionStart çalışmadıysa (ör. hook oturum ortasında eklendiyse) bu stop'u başlangıç say.
if [[ ! -f "$marker" ]]; then
  touch "$marker"
  exit 0
fi

changed="$(
  cd "$root" && find . -type f -newer "$marker" \
    -not -path './.git/*' \
    -not -path './.claude/.state/*' \
    -not -path '*/node_modules/*' \
    -not -path '*/vendor/*' \
    -not -path './ai-sessions/*' \
    -not -name '.DS_Store' \
    -not -name 'DECISIONS.md' \
    | sed 's|^\./||' | sort
)"

touch "$marker"

if [[ -z "$changed" ]]; then
  exit 0
fi

count="$(wc -l <<<"$changed" | tr -d ' ')"
list="$(head -n 15 <<<"$changed" | sed 's/^/- /')"
[[ "$count" -gt 15 ]] && list="$list"$'\n'"- … ve $((count - 15)) dosya daha"

reason="Görev kapanış kontrolü (CLAUDE.md › Karar kuralları §4). Son kontrolden beri değişen dosyalar ($count):
$list

Bitirmeden önce:
1. Bu değişikliklerde Kural 1 kapsamına giren ama kullanıcıya sorulmadan verilmiş bir seçim var mı? Varsa şimdi AskUserQuestion ile sor.
2. Onaylanan her karar DECISIONS.md'de kayıtlı mı ve indeks tablosu güncel mi? Değilse documenting-decisions skill'i ile işle.
3. Yapılan iş DECISIONS.md'deki 'Kabul edildi' bir kayıtla çelişiyor mu?

Üçü de temizse kullanıcıya tek satırda 'Karar kontrolü: temiz' yaz ve bitir."

jq -n --arg r "$reason" '{decision: "block", reason: $r}'
