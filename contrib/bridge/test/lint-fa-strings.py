"""Persian style lint for quoted strings in install-bridge.sh"""
import re, sys
p = sys.argv[1] if len(sys.argv) > 1 else 'install-bridge.sh'
hits = 0
for i, l in enumerate(open(p, encoding='utf-8'), 1):
    for m in re.finditer(r'"([^"]*[\u0600-\u06FF][^"]*)"', l):
        t = m.group(1)
        checks = {'tanween': '\u064b' in t, 'end-dot': bool(re.search(r'[.۔]\s*$', t)), 'excl': '!' in t,
                  'ra': bool(re.search(r'(^|\s)را(\s|$)', t)), 'mi': bool(re.search(r'(^|\s)(ن?می)[ \u200c]', t)),
                  'words': bool(re.search(r'جهت|انصراف|صرفا|ارسال مجدد|نمایید|فرمایید', t))}
        for k, v in checks.items():
            if v:
                hits += 1; print(i, k, t)
print("HITS", hits)
sys.exit(1 if hits else 0)
