import unittest

import acurl_cli


class TargetTest(unittest.TestCase):
    def test_platforms(self):
        v = acurl_cli.__version__
        self.assertEqual(acurl_cli.target("Darwin", "arm64"), (f"acurl_{v}_darwin_arm64.tar.gz", "acurl"))
        self.assertEqual(acurl_cli.target("Linux", "x86_64"), (f"acurl_{v}_linux_amd64.tar.gz", "acurl"))
        self.assertEqual(acurl_cli.target("Windows", "AMD64"), (f"acurl_{v}_windows_amd64.zip", "acurl.exe"))
        with self.assertRaisesRegex(RuntimeError, "no prebuilt binary"):
            acurl_cli.target("SunOS", "sparc")

    def test_expected_sum(self):
        sums = "aaa  acurl_1.0.0_linux_amd64.tar.gz\nbbb  acurl_1.0.0_darwin_arm64.tar.gz\n"
        self.assertEqual(acurl_cli.expected_sum(sums, "acurl_1.0.0_darwin_arm64.tar.gz"), "bbb")
        with self.assertRaisesRegex(RuntimeError, "not listed"):
            acurl_cli.expected_sum(sums, "nope")


if __name__ == "__main__":
    unittest.main()
