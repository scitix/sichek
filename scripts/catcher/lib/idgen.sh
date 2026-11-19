# shellcheck shell=bash
# idgen.sh — match canonical keys -> component IDs, assemble the full 机型ID,
# and render the human-readable config description (both from a live scan and
# from a bare ID string).

# ---- component matchers: echo the ID, or "" if key empty / not in catalog ----
match_cpu()  { [ -n "${1:-}" ] && echo "${CPU_ID[$1]:-}"  || true; }
match_gpu()  { [ -n "${1:-}" ] && echo "${GPU_ID[$1]:-}"  || true; }
match_mem()  { [ -n "${1:-}" ] && echo "${MEM_ID[$1]:-}"  || true; }
# Canonical NIC form: per-speed port TOTAL, ascending, "<spd>:<ports>+...".
# The spec writes the same config two ways ("400Gbps *8" -> 400Gx8*1 and
# "400Gbps物理卡(1口)*8" -> 400Gx1*8), so matching compares totals, not text.
nic_canon() {             # nic_canon "<spd>Gx<ports>*<cards>[+...]"
  local key="$1" seg spd ports cards out=""
  local -A tot=()
  local -a segs
  IFS='+' read -ra segs <<< "$key"
  for seg in "${segs[@]}"; do
    [[ "$seg" =~ ^([0-9]+)Gx([0-9]+)\*([0-9]+)$ ]] || continue
    spd="${BASH_REMATCH[1]}"; ports="${BASH_REMATCH[2]}"; cards="${BASH_REMATCH[3]}"
    tot[$spd]=$(( ${tot[$spd]:-0} + ports * cards ))
  done
  while read -r spd; do
    [ -n "$spd" ] || continue
    out="${out:+$out+}${spd}:${tot[$spd]}"
  done < <(printf '%s\n' "${!tot[@]}" | sort -n)
  printf '%s' "$out"
}

match_nic() {
  local want k
  want="$(nic_canon "${1:-}")"
  [ -z "$want" ] && return 0
  for k in "${!NIC_ID[@]}"; do
    [ "$(nic_canon "$k")" = "$want" ] && { printf '%s' "${NIC_ID[$k]}"; return 0; }
  done
}

# === 核心修改：硬盘匹配逻辑增强 ===
# 如果直接匹配实测容量失败，自动将 *1 的盘（通常是系统盘被 RAID1 遮蔽）乘以 2，
# 再次尝试匹配机型库。只要匹配上，就直接认可并返回该机型 ID。
match_disk() { 
  [ -z "${1:-}" ] && return
  local exact="${DISK_ID[$1]:-}"
  if [ -n "$exact" ]; then
    echo "$exact"
  else
    local guess
    guess="$(guess_disk_raid "$1")"
    [ -n "$guess" ] && echo "${guess%%|*}" || true
  fi
}
# ==================================

# Best-guess disk match when a RAID logical volume masks the boot pair: retry
# with each single SSD tier doubled (RAID1) and report if that hits the catalog.
# Echoes "<ID>|<assumed-key>" or "".
guess_disk_raid() {          # guess_disk_raid "<canonical key>"
  local key="$1" seg label n out=""
  IFS='+' read -ra segs <<< "$key"
  for idx in "${!segs[@]}"; do
    seg="${segs[$idx]}"; label="${seg%\**}"; n="${seg#*\*}"
    [ "$n" = "1" ] || continue
    local trial=() j
    for j in "${!segs[@]}"; do
      if [ "$j" = "$idx" ]; then trial+=("${label}*2"); else trial+=("${segs[$j]}"); fi
    done
    local tkey; tkey="$(IFS='+'; echo "${trial[*]}")"
    if [ -n "${DISK_ID[$tkey]:-}" ]; then echo "${DISK_ID[$tkey]}|$tkey"; return; fi
  done
}

# ---- split a full 机型ID string into component codes ----------------------
# IDs are concatenated tokens: I/A = CPU, G = GPU, N = NIC, M = MEM, S = DISK.
# CPU codes can end in a hex-ish letter (I3A); others are <letter><2 alnum>.
# We scan left to right and bucket by leading letter.
split_id() {                 # split_id "I31G82N23M65S03" -> sets SP_CPU SP_GPU SP_NIC SP_MEM SP_DISK
  SP_CPU=""; SP_GPU=""; SP_NIC=""; SP_MEM=""; SP_DISK=""
  local s="$1" tok
  # tokens are always a letter followed by exactly two [0-9A-Z]
  while [ -n "$s" ]; do
    tok="${s:0:3}"
    [ ${#tok} -lt 3 ] && break
    s="${s:3}"
    case "$tok" in
      I*|A*) SP_CPU="$tok" ;;
      G*)    SP_GPU="$tok" ;;
      N*)    SP_NIC="$tok" ;;
      M*)    SP_MEM="$tok" ;;
      S*)    SP_DISK="$tok" ;;
    esac
  done
}

# ---- NIC description helpers ----------------------------------------------
# Low speeds are ethernet (双口 cards = ports/2); high speeds are IB (单口 cards
# = ports). Generation from detected IB_GEN, else per-ID override, else default.
_eth_speed() { case "$1" in 1|10|25) return 0;; *) return 1;; esac; }
_ibgen_default() { case "$1" in 100) echo EDR;; 200) echo HDR;; 400) echo NDR;; 800) echo NDR;; *) echo "";; esac; }

# render_nic_desc <Nid> <"spd*n+..."> : prints 网卡/IB卡 lines from a canonical key
render_nic_desc() {
  local nid="$1" key="$2" seg spd ports n total out_eth="" out_ib="" gen
  local -A ports_by_spd=()                       # merge same-speed segments
  IFS='+' read -ra segs <<< "$key"
  for seg in "${segs[@]}"; do
    # catalog key form "<spd>Gx<ports>*<cards>"; plain "<spd>*<ports>" also accepted
    if [[ "$seg" =~ ^([0-9]+)Gx([0-9]+)\*([0-9]+)$ ]]; then
      spd="${BASH_REMATCH[1]}"; ports="${BASH_REMATCH[2]}"; n="${BASH_REMATCH[3]}"
    elif [[ "$seg" =~ ^([0-9]+)\*([0-9]+)$ ]]; then
      spd="${BASH_REMATCH[1]}"; ports=1; n="${BASH_REMATCH[2]}"
    else
      continue
    fi
    ports_by_spd[$spd]=$(( ${ports_by_spd[$spd]:-0} + ports * n ))
  done
  while read -r spd; do
    [ -n "$spd" ] || continue
    total="${ports_by_spd[$spd]}"                  # physical ports of this speed
    if _eth_speed "$spd"; then
      # spec writes "10Gbps*2+10Gbps*2+10Gbps*2" as three separate segments (N41);
      # keep one segment per 双口/单口 group instead of merging into "10G*6".
      local cards odd
      cards=$(( total / 2 )); odd=$(( total % 2 ))
      [ "$cards" -gt 0 ] && out_eth="${out_eth:+$out_eth, }${spd}G双口以太网卡*${cards}"
      [ "$odd" -gt 0 ]   && out_eth="${out_eth:+$out_eth, }${spd}G单口以太网卡*${odd}"
    else
      gen="${IB_GEN_LOCAL[$spd]:-}"                  # from live detect, if any
      [ -z "$gen" ] && gen="${NIC_IBGEN[${nid}:${spd}]:-}"
      [ -z "$gen" ] && gen="$(_ibgen_default "$spd")"
      out_ib="${out_ib:+$out_ib, }${gen:+$gen }${spd}G单口网卡*${total}"
    fi
  done < <(printf '%s\n' "${!ports_by_spd[@]}" | sort -n)
  [ -n "$out_eth" ] && printf '网卡: %s\n' "$out_eth"
  [ -n "$out_ib" ]  && printf 'IB卡: %s\n' "$out_ib"
}

# Live NIC render: uses the REAL physical Ethernet/IB split from detect_net
# (ETH_GROUPS / IB_GROUPS / IB_GEN), not the speed heuristic. Authoritative.
render_nic_desc_live() {
  local g out_eth="" out_ib="" gen
  while read -r g; do [ -z "$g" ] && continue
    local n cards odd
    n="${ETH_GROUPS[$g]}"
    if [ "$g" -ge 200 ]; then
      cards=0; odd=$n
    else
      cards=$(( n/2 )); odd=$(( n%2 ))
    fi
    [ "$cards" -gt 0 ] && out_eth="${out_eth:+$out_eth, }${g}G双口以太网卡*${cards}"
    [ "$odd" -gt 0 ]   && out_eth="${out_eth:+$out_eth, }${g}G单口以太网卡*${odd}"
  done < <(for g in "${!ETH_GROUPS[@]}"; do echo "$g"; done | sort -n)
  
  while read -r g; do [ -z "$g" ] && continue
    gen="${IB_GEN[$g]:-$(_ibgen_default "$g")}"
    out_ib="${out_ib:+$out_ib, }${gen:+$gen }${g}G单口网卡*${IB_GROUPS[$g]}"
  done < <(for g in "${!IB_GROUPS[@]}"; do echo "$g"; done | sort -n)
  
  [ -n "$out_eth" ] && printf '网卡: %s\n' "$out_eth"
  [ -n "$out_ib" ]  && printf 'IB卡: %s\n' "$out_ib"
}

# ---- DISK description helper ----------------------------------------------
# Smallest tier = system disk (SSD, raid note if 2-3 disks); rest = data disks.
render_disk_desc() {        # render_disk_desc <"label*n+..."> <Sid> [live-media-assoc-name]
  local key="$1" sid="${2:-}" seg label n first=1
  IFS='+' read -ra segs <<< "$key"
  local sys="" data=""
  for seg in "${segs[@]}"; do
    label="${seg%\**}"; n="${seg#*\*}"
    # media priority: live ROTA reading > source spec (HDD tags) > default SSD
    local media="${DISK_MEDIA_SPEC[${sid}|${label}]:-SSD}"
    [ -n "${3:-}" ] && { local -n M="$3"; media="${M[$label]:-$media}"; }
    if [ "$first" -eq 1 ]; then
      local note=""; { [ "$n" = "2" ] || [ "$n" = "3" ]; } && note="（需支持raid1）"
      sys="系统盘: ${media} ${label}*${n}${note}"
      first=0
    else
      data="${data:+$data + }${media} ${label}*${n}"
    fi
  done
  [ -n "$sys" ]  && printf '%s\n' "$sys"
  [ -n "$data" ] && printf '数据盘: %s\n' "$data"
}

# ===========================================================================
# describe_id — full 机型ID -> multi-line config. <ddr> optional (detect mode
# passes the real DDR type; pure-ID mode infers from CPU generation).
# Uses live IB_GEN_LOCAL / DETECTED_MEDIA assoc arrays if the caller exported them.
# ===========================================================================
describe_id() {            # describe_id "I31G82N23M65S03"
  split_id "$1"
  declare -gA IB_GEN_LOCAL  # may be empty
  local line ddr

  if [ -n "$SP_CPU" ]; then
    line="${CPU_DESC[$SP_CPU]:-}"
    [ -n "$line" ] && printf 'CPU: %s\n' "$line" || printf 'CPU: (未知代码 %s)\n' "$SP_CPU"
  fi
  if [ -n "$SP_MEM" ]; then
    line="${MEM_DESC[$SP_MEM]:-}"
    # only index CPU_MEMGEN when SP_CPU is non-empty (empty subscript errors under set -u)
    ddr="${DETECTED_DDR:-${SP_CPU:+${CPU_MEMGEN[$SP_CPU]:-}}}"
    if [ -n "$line" ]; then printf '内存: %s%s\n' "${ddr:+$ddr }" "$line"
    else printf '内存: (未知代码 %s)\n' "$SP_MEM"; fi
  fi
  if [ -n "$SP_DISK" ]; then
    # Prefer the canonical key (so we can split system/data); fall back to desc text.
    # sorted so alias keys (e.g. S06: "1.6T*1" + "1.7T*1") resolve deterministically
    local dkey=""; while read -r k; do [ -n "$k" ] && [ "${DISK_ID[$k]}" = "$SP_DISK" ] && { dkey="$k"; break; }; done < <(printf '%s\n' "${!DISK_ID[@]}" | LC_ALL=C sort)
    if [ -n "$dkey" ]; then
      if [ -n "${DETECTED_MEDIA_NAME:-}" ]; then render_disk_desc "$dkey" "$SP_DISK" "$DETECTED_MEDIA_NAME"
      else render_disk_desc "$dkey" "$SP_DISK"; fi
    elif [ -n "${DISK_DESC[$SP_DISK]:-}" ]; then
      # alias ID with no key of its own (e.g. S25 duplicates S32's config)
      printf '硬盘: %s\n' "${DISK_DESC[$SP_DISK]}"
    else printf '硬盘: (未知代码 %s)\n' "$SP_DISK"; fi
  fi
  if [ -n "$SP_GPU" ]; then
    line="${GPU_DESC[$SP_GPU]:-}"
    [ -n "$line" ] && printf 'GPU: %s\n' "$line" || printf 'GPU: (未知代码 %s)\n' "$SP_GPU"
  fi
  if [ -n "$SP_NIC" ]; then
    # nic_canon form ("25:2+200:2") -> the spec's original text, so describe
    # always prints the catalog wording verbatim (incl. N41's three 10G groups).
    local want k nkey=""
    want="$(nic_canon "$SP_NIC")"
    while read -r k; do
      [ -n "$k" ] || continue
      [ "${NIC_ID[$k]}" = "$SP_NIC" ] && { nkey="$k"; break; }
    done < <(printf '%s\n' "${!NIC_ID[@]}" | LC_ALL=C sort)
    if [ -n "${NIC_DESC[$SP_NIC]:-}" ]; then
      printf '网卡: %s\n' "${NIC_DESC[$SP_NIC]}"
    elif [ -n "$nkey" ]; then render_nic_desc "$SP_NIC" "$nkey"
    else printf '网卡: (未知代码 %s)\n' "$SP_NIC"; fi
  fi
}
