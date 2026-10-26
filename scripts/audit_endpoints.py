#!/usr/bin/env python3
"""核对本 SDK 是否跟上了 workbuddy-manager 的**最新端点**。

用法：
    python scripts/audit_endpoints.py /path/to/workbuddy-manager

退出码：0 = 全部覆盖（或只剩已声明的别名路径）；1 = 有未覆盖端点（可用于 CI / 发版检查）。

## 比较规则（被写坏过三次，教训都留在这里）

  * **第一次**：只做 `lit.startswith('/api')` —— 会把 SDK 里路由前缀分类用的常量
    `"/api"`（`transport.go` 的 `isAdminPath`）当成端点，于是 `/api/*` 全部假阳性；
  * **第二次**：放宽正则后收进裸 `"/"`，`target.startswith("/")` **恒真**，变成全部
    「已覆盖」；
  * **第三次**：按「静态前缀段」比较时，只要求 SDK 里有 `"/api/accounts"`，于是
    `/api/accounts/export`、`/api/accounts/refresh-plan/run` 这类**更深的**端点被
    前缀吃掉，报 0 未覆盖而实际有十几个没有具名方法。

现在的规则按端点形状分三种：

  1. **无路径参数**：SDK 必须有**完全相等**的字面量（如 `/api/accounts/export`）；
  2. **参数在末尾**：SDK 必须有到参数为止的前缀字面量（`/api/accounts/`）；
  3. **参数在中间**：除前缀外，参数之后的每个**静态段连续段**都要有对应字面量
     （`/api/accounts/{filename}/proxy` 需要 `"/proxy"`；`/api/proxy-routes/{name}/accounts`
     需要 `"/accounts"`）。SDK 用「前缀 + 逃逸后的值 + 尾巴」拼接，尾巴就是这里要的。

这样嵌套路径再也藏不住，同时不会把 `accountPath(file) + "/proxy"` 这种写法误判。
"""
from __future__ import annotations

import argparse
import pathlib
import re
import sys

# 只用于「路径前缀分类」的字面量，不是请求目标——必须排除，否则全是假阳性
NOISE = {'/', '/api', '/v1', '/v2', '/responses'}

# 已确认只做别名、且明确用逃生舱可达的端点（见 README）。列在这里是为了让
# 「未覆盖」列表只剩真正需要处理的东西。
KNOWN_ALIASES = {'/v2/chat/completions', '/responses'}


def segs(path: str) -> list[str]:
    return [s for s in path.strip('/').split('/') if s]


def is_param(seg: str) -> bool:
    # FastAPI 的 {name} / {name:path}；%s 是模板里可能出现的写法
    return seg.startswith('{') or seg in ('%s', '%d')


def split_endpoint(path: str) -> tuple[list[str], list[list[str]]]:
    """把端点拆成（静态前缀, 参数之后的静态段连续段们）。

    例：``/api/accounts/{filename}/proxy`` → (``[api, accounts]``, ``[[proxy]]``)；
    无参数的端点 → (``[api, accounts, export]``, ``[]``)。
    """
    all_segs = [s for s in path.strip('/').split('/') if s]
    prefix: list[str] = []
    runs: list[list[str]] = []
    seen_param = False
    run: list[str] = []
    for s in all_segs:
        if is_param(s):
            if seen_param and run:
                runs.append(run)
            run = []
            seen_param = True
            continue
        if seen_param:
            run.append(s)
        else:
            prefix.append(s)
    if run:
        runs.append(run)
    return prefix, runs


def missing_reasons(path: str, lits: list[list[str]]) -> list[str]:
    """返回该端点**缺少**的字面量（空列表 = 已覆盖）。"""
    prefix, runs = split_endpoint(path)
    lacks: list[str] = []
    if prefix and not any(ls == prefix for ls in lits):
        # 无参数时前缀就是整条路径，提示里说清楚是「整条」还是「前缀」
        lacks.append('/' + '/'.join(prefix) + ('/' if runs else ''))
    for run in runs:
        if not any(ls == run for ls in lits):
            lacks.append('/' + '/'.join(run))
    return lacks


def collect_endpoints(upstream_root: pathlib.Path) -> list[tuple[str, str, str]]:
    routers = upstream_root / 'server' / 'routers'
    eps: list[tuple[str, str, str]] = []
    for f in sorted(routers.glob('*.py')):
        src = f.read_text(encoding='utf-8')
        # 一个文件里可能有**多个** APIRouter（例如红包装在 redpackets.py 里又单独
        # 挂了一个公开的 claim_router）。只认第一个的话，后面那些 router 的端点会
        # 被静默漏检——这个脚本的一个教训就是「别相信印象，去解析」。
        # 注意有的 router 不写 prefix（路径自己带全，如 gateway.py），所以前缀取
        # 不到时按空串处理，而不是把这个 router 整个跳过。
        prefixes = {}
        for m in re.finditer(r"^(\w+)\s*=\s*APIRouter\((.*)\)", src, re.MULTILINE):
            pm = re.search(r"prefix\s*=\s*['\"]([^'\"]*)['\"]", m.group(2))
            prefixes[m.group(1)] = pm.group(1) if pm else ''
        for mm in re.finditer(
            r"@(\w+)\.(get|post|put|patch|delete)\(\s*['\"]([^'\"]*)['\"]", src
        ):
            var, method, path = mm.group(1), mm.group(2), mm.group(3)
            if var not in prefixes:
                continue
            eps.append((method.upper(), prefixes[var] + path, f.name))
    return eps


def collect_sdk_literals(sdk_root: pathlib.Path) -> list[list[str]]:
    lits: list[list[str]] = []
    for f in sorted(sdk_root.glob('*.go')):
        if f.name == 'doc.go' or f.name.endswith('_test.go'):
            continue
        for mm in re.finditer(r'"(/[^"]*)"', f.read_text(encoding='utf-8')):
            if mm.group(1) not in NOISE:
                lits.append(segs(mm.group(1)))
    return lits


def main() -> int:
    ap = argparse.ArgumentParser(description='核对 SDK 与上游端点的覆盖差异')
    ap.add_argument('upstream', help='workbuddy-manager 仓库根目录')
    args = ap.parse_args()

    upstream_root = pathlib.Path(args.upstream).resolve()
    if not (upstream_root / 'server' / 'routers').is_dir():
        print(f'找不到上游路由目录：{upstream_root / "server" / "routers"}', file=sys.stderr)
        return 2

    sdk_root = pathlib.Path(__file__).resolve().parent.parent
    eps = collect_endpoints(upstream_root)
    lits = collect_sdk_literals(sdk_root)

    missed: list[tuple[str, str, str, list[str]]] = []
    for method, path, fname in eps:
        lacks = missing_reasons(path, lits)
        if lacks:
            missed.append((method, path, fname, lacks))

    print(f'上游端点 {len(eps)}，SDK 字面量（按段，去噪）{len(lits)}，未覆盖 {len(missed)}')

    unexpected = []
    for method, path, fname, lacks in missed:
        tag = '（已声明的别名，逃生舱可达）' if path in KNOWN_ALIASES else '  ← 需要处理'
        if path not in KNOWN_ALIASES:
            unexpected.append(path)
        print(f'  {method:6} {path:44} ({fname}){tag}')
        if path not in KNOWN_ALIASES:
            print(f'         缺字面量: {", ".join(lacks)}')

    return 1 if unexpected else 0


if __name__ == '__main__':
    raise SystemExit(main())
