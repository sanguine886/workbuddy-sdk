"""端点审计：嵌套路径不能假阳性，指定版本不能混入工作区改动。"""
from __future__ import annotations

import pathlib
import subprocess
import tempfile
import unittest

import audit_endpoints as audit


class EndpointAuditTest(unittest.TestCase):
    def test_nested_paths_need_complete_literals(self) -> None:
        literals = [audit.segs('/api/accounts')]
        self.assertTrue(audit.missing_reasons('/api/accounts/export', literals))
        self.assertTrue(audit.missing_reasons('/api/accounts/{file}/proxy', literals))
        literals.append(audit.segs('/proxy'))
        self.assertFalse(audit.missing_reasons('/api/accounts/{file}/proxy', literals))

    def test_ref_reads_committed_routes_without_changing_worktree(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = pathlib.Path(tmp)

            def git(*args: str) -> str:
                return subprocess.check_output(
                    ['git', '-C', tmp, *args], encoding='utf-8', stderr=subprocess.PIPE,
                ).strip()

            git('init', '-q')
            routes = root / 'server' / 'routers'
            routes.mkdir(parents=True)
            source = routes / 'sample.py'
            source.write_text(
                "router = APIRouter(prefix='/api/items')\n"
                "public = APIRouter()\n"
                "@router.get('')\n"
                "@router.post('/{id}/probe')\n"
                "@public.get('/healthz')\n", encoding='utf-8',
            )
            git('add', '.')
            git('-c', 'user.name=SDK test', '-c', 'user.email=sdk@example.invalid',
                '-c', 'commit.gpgsign=false', 'commit', '-qm', 'fixture')
            source.write_text("router = APIRouter(prefix='/uncommitted')\n"
                              "@router.get('')\n", encoding='utf-8')
            before = git('status', '--porcelain')
            self.assertEqual(audit.collect_endpoints(root, 'HEAD'), [
                ('GET', '/api/items', 'sample.py'),
                ('POST', '/api/items/{id}/probe', 'sample.py'),
                ('GET', '/healthz', 'sample.py'),
            ])
            self.assertEqual(audit.collect_endpoints(root), [
                ('GET', '/uncommitted', 'sample.py'),
            ])
            self.assertEqual(before, git('status', '--porcelain'))


if __name__ == '__main__':
    unittest.main()
