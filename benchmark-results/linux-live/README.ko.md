# Linux live USB 실행 키트

Ubuntu live USB처럼 아무것도 없는 Linux에서 기존 `encoding/xml`과 `GOEXPERIMENT=xmlv2`를 같은 16 MiB 문서로 비교한다. 측정 내용은 [`../darwin-arm64`](../darwin-arm64/README.md)와 같다. 가상화가 없으므로 WSL 결과(`../linux-amd64`)의 오버헤드가 없다.

## 준비

- Ubuntu 24.04 Desktop ISO를 USB에 굽는다(Rufus, balenaEtcher, Ventoy 등).
- 결과를 옮길 USB 저장장치를 하나 더 준비한다. live 세션의 파일은 RAM에만 있어서 끄면 사라진다. Windows 파티션은 빠른 시작이나 BitLocker 때문에 읽기 전용으로 잡힐 수 있다.
- USB로 부팅해서 "Try Ubuntu"를 고르고, 네트워크(유선 또는 Wi-Fi)를 연결한다.
- 측정 중에는 다른 프로그램을 열지 않는다.

## 실행

터미널(Ctrl+Alt+T)에서:

```sh
cd ~
wget -O kit.tgz https://codeload.github.com/Clickin/go-xml-v2/tar.gz/refs/heads/benchmarks/windows-linux-amd64
tar -xzf kit.tgz
bash go-xml-v2-benchmarks-windows-linux-amd64/benchmark-results/linux-live/run.sh
```

순서대로 하는 일:

1. Go 1.26.0(이 트리의 최소 bootstrap)을 받고 SHA-256을 확인한다.
2. `xml-v2-poc` c759c45a38 소스를 받아 툴체인을 빌드한다(몇 분).
3. 16 MiB 문서를 `gen16mib.py`로 만들고 SHA-256을 확인한다. darwin/arm64 측정과 같은 파일이다.
4. 하네스를 두 번 빌드한다. `GOEXPERIMENT`를 비운 기존 빌드와 `GOEXPERIMENT=xmlv2` 빌드다.
5. CPU governor를 `performance`로 바꾸고(`sudo -n`, 안 되면 건너뜀) 성능 코어(`/sys/devices/cpu_core/cpus`, 13600K는 보통 0-11)에 고정한다. 끝나면 되돌린다.
6. 20라운드를 돈다. 라운드마다 두 빌드를 별도 프로세스로 돌리고, 먼저 도는 빌드를 번갈아 바꾼다. 프로세스마다 케이스별로 워밍업 3회, 측정 10회다.
7. 모든 리더의 이벤트 수와 체크섬(967,965 / 36,104,832)을 확인하고 `summary.md`, `benchstat.txt`를 만든다.

다운로드는 약 100 MB다. 빌드와 측정을 합쳐 15분 안팎으로 예상한다. 이 머신에서 재 본 시간은 아니다.

옵션은 환경 변수로 준다. 예: `ROUNDS=30 NO_TURBO=1 bash .../run.sh`

| 변수 | 기본값 | 뜻 |
|---|---|---|
| `ROUNDS` | 20 | 프로세스 쌍 개수 |
| `WARMUPS` | 3 | 프로세스·케이스당 워밍업 |
| `RUNS` | 10 | 프로세스·케이스당 측정 횟수 |
| `NO_TURBO` | 0 | `1`이면 측정 중 터보를 끈다 |
| `WORK` | `~/xmlbench` | 다운로드·빌드·결과 디렉터리 |

## 끝나면

마지막에 출력되는 `~/xmlbench/xmlbench-results-<UTC 시각>.tar.gz`를 파일 앱으로 USB 저장장치에 복사한다. 이 파일만 있으면 결과 README와 이슈 수치를 정리할 수 있다.
