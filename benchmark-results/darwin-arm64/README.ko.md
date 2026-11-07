# encoding/xml/v2 런타임 간 읽기 성능 비교 (arm64: 2026-11-06, HEAD bd51491007 / amd64: 2026-11-07, HEAD 02aa698c64)

기준선 (v2 API 대상):
- **하한선:** stax-xml (Node)
- **권장안:** Woodstox (Java). 최소한 이 수준은 나와야 한다.
- **목표안:** quick-xml (Rust)

v1 API(`encoding/xml`)는 기준선 판정 대상이 아니다. 토큰마다 값을 만들어 인터페이스에 담는 설계를 유지하는 한 성능 한계가 있어서, 현재 stdlib 대비 개선 폭만 참고로 적는다.

## 결론

| Go 경로 | 하한선 stax-xml | 권장안 Woodstox | 목표안 quick-xml (검사 켬) | quick-xml (원본 바이트) |
|---|---|---|---|---|
| `xmltext` (v2 토크나이저) | **통과** (4.97배) | **통과** (1.41배) | **통과** (1.08배) | 미달 (0.86배) |
| (참고) `encoding/xml` Token, xmlv2 빌드 | 2.27배 | 0.64배 | 0.50배 | 0.39배 |
| (참고) `encoding/xml` Token, 현재 stdlib | 0.94배 | — | — | — |

- **v2 토크나이저(`xmltext`)는 목표안을 넘었다.** 같은 일을 하게 한 quick-xml(검사 켬)보다 8% 빠르고, 최댓값(32.95ms)이 quick-xml 최솟값(34.98ms)보다 작다. Woodstox보다는 1.41배 빠르다.
  - 기계 상태 변화를 빼려고 두 바이너리를 번갈아 8번씩 돌린 측정(각 40회, a1626332cc)에서도 `xmltext` 30.4ms, quick-xml(검사 켬) 33.4ms로 1.10배였다. 검사 없는 quick-xml은 26.4ms로 `xmltext`의 1.15배 빠르다.
- **검사를 하지 않는 quick-xml(원본 바이트)과는 1.16배 차이다.** 이 설정은 UTF-8 확인, 참조 해석, 끝 태그 이름 대조를 하지 않고 이름도 문자열로 만들지 않는다.
- **목표안을 넘은 것은 이중 스캔 통합 덕분이다**(72999efa3c, e03e0080da, 75affe4032). 이전에는 경계 엔진이 바이트를 한 번 훑고 파서가 같은 바이트를 다시 훑었다. 이제 흔한 토큰(텍스트, 시작·끝 태그)은 한 번에 읽는다. 아래 "이전 측정과 달라진 점" 참고.
- **v1 API는 참고 수치다.** xmlv2 빌드에서 `encoding/xml`의 `Token()`은 현재 stdlib보다 2.41배, `Unmarshal`은 2.56배 빠르다. 현재 stdlib은 하한선 아래에 있다. `Token()`이 Woodstox의 0.64배에 머무는 것은 v1 API가 토큰마다 `StartElement`, `[]Attr`, 문자열을 새로 만들고 인터페이스에 담기 때문이다. 이 비용은 API를 바꾸지 않고는 없앨 수 없다. v1 경로는 호환 프로필이라 이중 스캔 통합의 대상이 아니다. 다만 e03e0080da가 공유 텍스트 커널을 바꿔 v1이 0.7–1.1% 느려졌고, a1626332cc에서 커널을 되돌려 64a589a0b2 대비 변화가 없어졌다(±0.3% 이내). 위 표의 v1 수치는 a1626332cc 이전 빌드라 그만큼 보수적이다.
- **amd64에서도 큰 문서는 Woodstox와 quick-xml을 모두 넘는다.** 작은 문서에서는 Woodstox보다 빠르고 quick-xml보다 느리다. 아래 "amd64" 참고.
- **이번 세션은 기계 전체가 느렸다**(Woodstox 42.7 → 45.7ms, quick-xml 검사 켬 31.7 → 35.3ms). 같은 세션 안의 비율만 비교한다.

## 방법

- **고정 입력.** stax-xml 벤치마크의 `test-data/runtime-comparison-16mib.xml`(16,777,038바이트, ASCII, 네임스페이스·엔티티 없음)이다.
- **같은 규칙.**
  - 측정 1회에 파일 읽기와 파싱이 모두 들어간다.
  - 시작·끝 요소 이름, 모든 속성 이름과 값, 공백이 아닌 텍스트 구간을 하나의 체크섬으로 접는다.
  - 모든 구현이 같은 이벤트 수(967,965)와 체크섬(36,104,832)을 냈다. 결과가 다르면 비교가 무효가 되도록 확인했다.
- **실행.** 모든 런타임을 한 세션에서 차례로 돌렸다. 각각 워밍업 3회, 측정 10회, 중앙값(Apple M4, 16GB, arm64).
- **다른 런타임.** `~/github/stax-xml-patch-1.1.1/packages/benchmark/reader-cross-runtime.mjs`를 그대로 실행했다.
  - stax-xml: 워크스페이스 패키지 1.1.2, Node 26.13.0
  - Woodstox: 6.7.0, OpenJDK 25.0.5
  - quick-xml: 0.40.1, Rust 1.98.1
- **quick-xml (검사 켬).** `quickxml-checked/`. 위 하네스와 같되 `xmltext`와 같은 일을 하게 했다.
  - 이름: `std::str::from_utf8`로 UTF-8 확인
  - 속성 값: `unescape_value()`로 참조 해석
  - 텍스트·CDATA: `decode()`, 일반 참조는 `resolve_char_ref`/`resolve_xml_entity`로 해석
  - `check_end_names`: 기본값(켬)
  - 여전히 `xmltext`보다 덜 하는 일: XML 이름 문자 규칙, 문자 범위(Char) 검사, 중복 속성 검사.
- **Go.** 이 디렉터리의 `main.go`, `v2.go`로 쟀다. 체크섬 규칙은 Rust 구현과 같다. 툴체인은 go-xml-v2 트리(go1.28-devel)이고 `GOGC`는 기본값이다.

## 결과

### 풀 리더 (같은 일을 하는 비교)

| 리더 | 처리량(중앙값) | 시간 중앙값 | 최소–최대 | 할당 수 / 1회 |
|---|---:|---:|---:|---:|
| quick-xml (Rust, 원본 바이트) | 572.6 MiB/s | 27.94 ms | 27.72–28.39 | — |
| **Go `xmltext` ReadToken** | **492.1 MiB/s** | **32.51 ms** | 32.07–32.95 | 65 |
| quick-xml (Rust, 검사 켬) | 453.6 MiB/s | 35.27 ms | 34.98–35.70 | — |
| Woodstox (Java) | 350.0 MiB/s | 45.72 ms | 45.22–46.99 | — |
| Go `encoding/xml` Token (xmlv2) | 225.1 MiB/s | 71.07 ms | 70.72–71.47 | 1,360,749 |
| stax-xml (Node) | 98.9 MiB/s | 161.74 ms | 158.67–170.10 | — |
| Go `encoding/xml` Token (현재 stdlib) | 93.4 MiB/s | 171.33 ms | 170.55–172.70 | 3,125,806 |

### amd64 (Intel i5-13600K, Windows 11과 WSL2 Ubuntu)

같은 세션 안의 처리량 비율(Go `xmltext` ÷ 다른 구현, 1보다 크면 Go가 빠름)이다. 02aa698c64 빌드, `GOAMD64=v1`, 9라운드(라운드마다 구현별 프로세스 하나), 라운드별 비율의 중앙값이다. 입력은 stax-xml 자산 8개(LF 원본)이고, 각 구현이 이벤트 수와 체크섬을 같게 낸다. Woodstox 7.2.0, quick-xml 0.42.0이다. quick-xml은 속성 값에 `normalized_value`, 텍스트에 `xml10_content`를 쓰므로 위 arm64 표의 "quick-xml (검사 켬)"과 설정이 다르다.

| 입력 | 측정 | Windows ÷Woodstox | Windows ÷quick-xml | WSL ÷Woodstox | WSL ÷quick-xml |
|---|---|---:|---:|---:|---:|
| midsize (13.6 MB) | 파일 읽기 포함 | 1.16 | 1.11 | 1.20 | 1.04 |
| large (101.9 MB) | 파일 읽기 포함 | 1.13 | 1.11 | 1.14 | 1.09 |
| 작은 문서 6개 (0.3–4.4 KB) | 파싱만 | 1.05–2.03 | 0.74–0.93 | 1.28–2.62 | 0.59–0.89 |
| 작은 문서 6개 | 파일 읽기 포함 | 1.21–1.35 | 0.85–0.90 | 2.39–5.00 | 0.50–0.76 |
| 합성 16 MiB 카탈로그 (메모리, 길이 합, 3프로세스) | 파싱만 | 1.13 | 1.25 | — | — |

- **큰 문서:** 권장안(Woodstox)과 목표안(quick-xml)을 모두 넘는다. arm64 결론과 방향이 같다.
- **작은 문서:** Woodstox보다 빠르고 quick-xml보다 느리다.
  - WSL에서 파일 읽기를 포함한 차이의 65–76%는 Go `os.ReadFile`이 Rust `fs::read`보다 문서당 약 2 µs 더 드는 데서 온다. `xmltext` 밖의 비용이다.
  - 나머지는 문서마다 드는 고정 비용(디코더 생성)과 네임스페이스 경로(ptest)다.
- **02aa698c64(이름 캐시를 이름 128개 뒤에 만들기)의 효과:** 이름이 32개를 넘는 작은 문서에서 파싱만 잰 시간이 WSL 1.05–1.21배, Windows 1.03–1.16배 빨라졌다. 큰 문서는 범위가 1을 포함한다(변화 없음).
- **편차:** 작은 문서는 한 번에 µs 단위라 라운드 사이 편차가 크다. 성능 코어에 고정해도 줄지 않았다. 라운드별 범위가 1을 걸치지 않는 차이만 결론에 썼다.
- **원시 결과:** GitHub `Clickin/go-xml-v2`(비공개) 브랜치 `benchmarks/windows-linux-amd64`의 `benchmark-results/`. `assets-comparison`, `linux-amd64`, `small-ab`(변경 전후 비교와 파일 읽기 분리)가 있다.

### 참고: 구조체 Unmarshal

풀 리더보다 하는 일이 많아서(리플렉션, 필드 대응, 문자열 보관) 위 표와 직접 비교할 수 없다. 세 경우 모두 같은 결과(값 56,939개, 필드 체크섬 1,322,845,707)를 냈다.

| 경로 | 처리량 | 시간 중앙값 | 할당 수 / 1회 |
|---|---:|---:|---:|
| `encoding/xml/v2` Unmarshal | 320.1 MiB/s | 49.98 ms | 563,589 |
| `encoding/xml` Unmarshal (xmlv2) | 179.1 MiB/s | 89.34 ms | 1,024,987 |
| `encoding/xml` Unmarshal (현재 stdlib) | 70.1 MiB/s | 228.37 ms | 4,600,377 |

v2 Unmarshal은 구조체까지 채우면서도 Woodstox 풀 리더의 91% 속도가 나온다. v2 Unmarshal의 할당은 필드에 저장하는 문자열과 슬라이스 증가분이 대부분이다.

### 이전 측정과 달라진 점

| 경로 | 5e9aedcbff | f9ac0580fe | 5fd7795e11 | 64a589a0b2 | 75affe4032 (지금) |
|---|---:|---:|---:|---:|---:|
| `xmltext` ReadToken | 362.3 MiB/s | 401.6 MiB/s | 398.9 MiB/s | 416.9 MiB/s | 492.1 MiB/s |
| `encoding/xml/v2` Unmarshal | 251.3 MiB/s | 270.1 MiB/s | 285.1 MiB/s | 285.1 MiB/s | 320.1 MiB/s |
| (같은 세션) Woodstox | — | 376.4 MiB/s | 374.8 MiB/s | 356.8 MiB/s | 350.0 MiB/s |
| (같은 세션) quick-xml 검사 켬 | — | 504.3 MiB/s | 504.2 MiB/s | 484.0 MiB/s | 453.6 MiB/s |

세션마다 기계 상태가 달라서(같은 세션의 Woodstox·quick-xml이 함께 움직인다) 열끼리는 비율로 본다. 커밋별 효과는 같은 기계 상태에서 앞뒤 바이너리를 번갈아 돌려 쟀다.

| 커밋 | 내용 | `xmltext` | v2 Unmarshal |
|---|---|---:|---:|
| e74a066f8a | 모든 Decoder에 이름 캐시 | 할당 967,988 → 66 | — |
| f9ac0580fe | 필드·원소 arshaler 1회 조회 | — | −9.4% |
| 5fd7795e11 | 슬라이스 제자리 증가 | — | 할당 −17% |
| e24e70b531 | 속성 이름을 표로 한 번에 검사 | −4.6% | −3.7% |
| 64a589a0b2 | 끝 태그가 시작 태그의 이름 문자열 재사용 | −1.7% | — |
| 72999efa3c | 흔한 토큰은 경계 배치 없이 바로 찾기 | −6.0% | −3.2% |
| e03e0080da | 텍스트를 찾으면서 검사 | −0.9% | −1.2% |
| 75affe4032 | 시작 태그를 한 번에 읽기 | −13.2% | −9.2% |
| a1626332cc | 공유 커널에서 `<` 검사를 빼고 텍스트 전용 검사로 | −1.6% | −1.0% |
| bd51491007 | 비ASCII 이름, 참조·공백·비ASCII가 든 속성 값도 한 번에 읽기 | 변화 없음 (이 문서는 해당 모양이 없음) | −3% 안팎 |
| 02aa698c64 | 4 KiB 이름 캐시를 이름 32개가 아니라 128개 뒤에 만들기 | 변화 없음 (16 MiB 문서) | 변화 없음. 새 디코더로 읽는 작은 문서는 amd64에서 −3~−17% |

v1 경로(9d6c1068d1 ASCII 이름 검사 표, 9e3dc48ca2 메서드 없는 타입은 검사 생략)는 위 결론의 참고 수치에 반영돼 있다.

## 목표안과의 남은 차이

이중 스캔 통합 뒤 `xmltext`(32.5ms)는 quick-xml(검사 켬, 35.3ms)보다 빠르다. 검사를 하지 않는 quick-xml(27.9ms)과의 차이 4.6ms는 대부분 하는 일의 차이다.

| 항목 | `xmltext` 시간 중 비중 (64a589a0b2 기준 측정) | 성격 |
|---|---:|---|
| 이름을 문자열로 돌려주기 (`RawName`의 intern) | 약 9–11% | v2 API 비용. quick-xml은 입력 바이트를 빌려준다. Woodstox도 intern한다. |
| 텍스트 검사 전체 (UTF-8, Char 범위, `&`, `]]>`) | 최대 6.4% | 원본 바이트 quick-xml은 하지 않는다. |
| 중복 속성 검사 | 1.3% | quick-xml은 하지 않는다. |

- **통합은 흔한 모양에만 적용된다.** 엄격 프로필이고 SplitText가 꺼져 있을 때, 텍스트·끝 태그·시작 태그가 버퍼 안에서 끝날 때다. 시작 태그는 bd51491007부터 비ASCII 이름과 참조·공백·비ASCII가 든 따옴표 값까지 포함한다. 그 밖의 토큰(주석, CDATA, PI, DOCTYPE, 오류가 있는 토큰, 버퍼 끝에 걸친 토큰, 속성 31개 이상)은 `xmltext`의 경계 엔진 경로를 그대로 쓴다. 경계 엔진을 거치는 토큰은 전체 바이트 입력에서 0~0.003%, reader 입력에서 0.6~1.6%다(버퍼를 다시 채울 때마다 약 32개).
- **검증:** `TestDirectMatchesEngine`과 `FuzzDirect`가 같은 입력을 직접 경로를 켜고 끈 채로 읽어 토큰, 이름, 속성, 위치, 깊이, 오류(메시지와 위치)를 비교한다. 입력 방식은 바이트 전체, 1바이트 reader, 7바이트 reader, 작은 MaxTokenSize, 작은 버퍼다. 문자 집합 reader로 중간에 입력이 바뀌는 경우도 포함한다. 퍼징은 이 대상만 약 13분 돌렸다. 커널 버그는 두 경로가 같이 틀릴 수 있어서, 독립 구현과 비교하는 `FuzzPermissive`, `FuzzUnmarshal`도 각 60초 돌렸다.
- **다른 모양의 문서(xmltext 패키지 벤치마크, 64a589a0b2 대비):** geomean −12%, 태그가 많은 문서 −24~−28%, 참조가 많은 문서는 변화 없음 또는 −6.7%, 순수 텍스트(ASCII, 한국어)는 변화 없음. bd51491007에서 한글 태그 이름 문서 −15~−18%, 참조가 든 속성 값 문서 −24~−26%(직접 경로를 끈 것과 비교해도 같은 폭).

## 한계

- **입력이 하나다.** 네임스페이스, 엔티티, 큰 텍스트, 긴 속성 목록이 많은 문서에서는 순위가 달라질 수 있다.
- **런타임별 조건이 다르다.** JVM은 기본 설정, Go는 기본 `GOGC`다. 측정 10회는 같은 프로세스 안에서 했다(JIT·캐시가 데워진 상태).
- **arm64 표는 Apple M4 한 대, amd64 표는 Intel i5-13600K 한 대에서 쟀다.** 두 기계는 입력, Woodstox·quick-xml 버전과 설정이 달라서 비율도 기계끼리 직접 비교하지 않는다.

## 다시 실행하기

```sh
F=~/github/stax-xml-patch-1.1.1/packages/benchmark/test-data/runtime-comparison-16mib.xml
OUT=$PWD/results
# Node / Java / Rust
(cd ~/github/stax-xml-patch-1.1.1/packages/benchmark && node reader-cross-runtime.mjs --runs 10 --warmups 3 \
  --json-out $OUT/others.json --md-out $OUT/others.md)
# quick-xml (검사 켬)
cargo run -q --release --offline --manifest-path quickxml-checked/Cargo.toml -- $F 3 10 > results/quickxml-checked.json
# Go (이 디렉터리에서)
GOEXPERIMENT=xmlv2 ../bin/go run . -file $F -warmups 3 -runs 10 > results/go-xmlv2.json
../bin/go run . -file $F -warmups 3 -runs 10 > results/go-none.json
```

원시 결과: `results/others.json`, `results/quickxml-checked.json`, `results/go-xmlv2.json`, `results/go-none.json`.
