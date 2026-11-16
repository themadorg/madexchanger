import re,sys
p=sys.argv[1]; incode=False; hits=0
for i,l in enumerate(open(p,encoding='utf-8'),1):
    s=l.rstrip('\n')
    if s.startswith('```'): incode=not incode; continue
    if incode: continue
    t=re.sub(r'`[^`]*`','',s)  # drop inline code
    t2=re.sub(r'\[[^\]]*\]\([^)]*\)','',t)
    checks={'tanween':'\u064b' in s,'end-dot':bool(re.search(r'[.۔]\s*$',t2)),
            'mid-dot':bool(re.search(r'[\u0600-\u06FF][.۔]\s',t2)),'excl':'!' in t2,
            'ra':bool(re.search(r'(^|\s)را(\s|$)',t)),'mi-space':bool(re.search(r'(^|\s)(ن?می)[ \u200c]',t)),
            'jahat/ensraf/etc':bool(re.search(r'جهت|انصراف|صرفا|ارسال مجدد|نمایید|فرمایید',t))}
    for k,v in checks.items():
        if v: hits+=1; print(f"{i}:{k}: {s}")
print("TOTAL HITS",hits)
