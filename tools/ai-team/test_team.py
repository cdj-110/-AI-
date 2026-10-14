import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from team import Workflow, safe_file, publish

class WorkflowTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.repo = self.root / 'repo'
        self.repo.mkdir()
        self.git('init')
        (self.repo / 'README.md').write_text('baseline\n')
        self.git('add', 'README.md')
        self.git('-c', 'user.name=Test', '-c', 'user.email=test@example.invalid', 'commit', '-m', 'baseline')
    def tearDown(self):
        # Windows Git object files may be read-only; TemporaryDirectory handles them.
        self.tmp.cleanup()
    def git(self, *args):
        return subprocess.check_output(['git', *args], cwd=self.repo, stderr=subprocess.PIPE)
    def workflow(self, path='docs/ai-tasks/result.md', approved=True, profile='docs'):
        job = self.root / 'job'
        job.mkdir()
        def provider(role, payload):
            return {'plan': {'summary': 'test', 'steps': ['write']},
                    'develop': {'summary': 'test', 'files': [{'path': path, 'content': '# Acceptance\n\nChecks must pass before human review.\n'}]},
                    'review': {'approved': approved, 'findings': []}}[role]
        return Workflow({'repository': str(self.repo)}, {'title': 'fixture', 'profile': profile, 'request': 'fixture'}, job, provider)
    def test_path_boundaries(self):
        for path in ['../outside.md', 'docs/ai-tasks/../../x.md', 'C:/secret.md', '.github/workflows/x.md', 'docs/ai-tasks/.env', 'docs\\ai-tasks\\x.md']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                safe_file(self.repo, path, 'docs')
    def test_fixture_is_explicit_and_original_unchanged(self):
        result = self.workflow().run()
        self.assertEqual(result['mode'], 'test_fixture')
        self.assertEqual(result['status'], 'awaiting_human_publication')
        self.assertFalse((self.repo / 'docs').exists())
        self.assertEqual(self.git('status', '--porcelain').strip(), b'')
    def test_rejection_fails_closed(self):
        workflow = self.workflow(approved=False)
        with self.assertRaises(RuntimeError): workflow.run()
        self.assertEqual(workflow.record['status'], 'failed')
    def test_missing_code_harness_cannot_pass(self):
        workflow = self.workflow(path='apps/web/src/example.ts', profile='web')
        with self.assertRaisesRegex(RuntimeError, 'digest-pinned'): workflow.run()
        self.assertEqual(workflow.record['status'], 'failed')
    def test_fixture_cannot_publish(self):
        workflow = self.workflow()
        workflow.run()
        with self.assertRaisesRegex(ValueError, 'validated live candidate'):
            publish({'state_dir':str(self.root)}, 'job')
    def test_changed_candidate_cannot_publish(self):
        workflow = self.workflow()
        workflow.run()
        record = workflow.record
        record['mode'] = 'live_codex'
        (workflow.dir/'status.json').write_text(json.dumps(record), encoding='utf-8')
        with self.assertRaisesRegex(ValueError, 'Candidate changed'):
            publish({'state_dir':str(self.root)}, 'job')

if __name__ == '__main__': unittest.main()
