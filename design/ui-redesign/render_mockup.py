from pathlib import Path
from html import escape

OUT = Path(__file__).parent
parts = []
def rect(x,y,w,h,fill,rx=0,stroke='none'):
    parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{rx}" fill="{fill}" stroke="{stroke}"/>')
def text(x,y,s,size=14,fill='#303B4D',weight=400,mono=False):
    font='Menlo,monospace' if mono else 'PingFang SC,Hiragino Sans GB,Arial,sans-serif'
    parts.append(f'<text xml:space="preserve" x="{x}" y="{y}" font-family="{font}" font-size="{size}" font-weight="{weight}" fill="{fill}">{escape(s)}</text>')
def line(x,y,x2,y2,color='#E6EAF0'):
    parts.append(f'<path d="M{x} {y} L{x2} {y2}" stroke="{color}" fill="none"/>')
def dot(x,y,r,color):
    parts.append(f'<circle cx="{x}" cy="{y}" r="{r}" fill="{color}"/>')
def icon(x,y,kind,color='#8290A2'):
    paths={'server':'M2 2h16v7H2z M2 13h16v7H2z M5 5h2 M5 16h2',
           'folder':'M1 5h7l2 3h11v12H1z',
           'file':'M4 1h9l5 5v15H4z M13 1v6h5',
           'search':'M15 15l6 6 M17 9a7 7 0 1 1-14 0a7 7 0 1 1 14 0',
           'plus':'M11 3v16 M3 11h16',
           'chevron':'M6 8l5 5l5-5',
           'spark':'M11 1l3 7l7 3l-7 3l-3 7l-3-7l-7-3l7-3z',
           'split':'M2 3h18v17H2z M11 3v17',
           'send':'M3 12L11 4l8 8 M11 4v16',
           'settings':'M11 5a6 6 0 1 0 0 12a6 6 0 1 0 0-12 M11 1v4 M11 17v4 M1 11h4 M17 11h4',
           'sun':'M11 6a5 5 0 1 0 0 10a5 5 0 1 0 0-10 M11 0v3 M11 19v3 M0 11h3 M19 11h3',
           'upload':'M3 15v6h17v-6 M11 16V2 M5 8l6-6l6 6',
           'refresh':'M19 7a8 8 0 1 0 0 9 M19 1v6h-6'}
    parts.append(f'<path transform="translate({x} {y})" d="{paths[kind]}" fill="none" stroke="{color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>')

parts.append('<svg xmlns="http://www.w3.org/2000/svg" width="1680" height="1080" viewBox="0 0 1680 1080">')
rect(0,0,1680,1080,'#ECF0F5')
text(44,46,'NexShell',23,weight=650)
text(166,45,'界面改版 · 方案 01',15,'#768296')
text(1320,45,'浅色工作台 / 深色终端',15,'#768296')
# Window and chrome
rect(32,76,1616,940,'#FFFFFF',12,'#D8DFE9')
rect(33,77,1614,49,'#FAFBFD',12)
rect(33,108,1614,18,'#FAFBFD')
for x,c in [(55,'#F27770'),(75,'#EDC361'),(95,'#6AC48E')]: dot(x,101,6,c)
text(123,107,'›_',20,'#3974DC',650,True)
text(157,106,'NexShell',15,weight=600)
text(749,106,'服务器工作台',13,'#8A94A4')
icon(1562,90,'sun'); icon(1600,90,'settings')
line(33,126,1647,126)
# Sidebar
rect(33,127,257,857,'#F5F7FA')
line(290,127,290,984)
text(55,165,'连接',19,weight=600)
icon(246,146,'plus','#617087')
rect(51,184,221,36,'#FFFFFF',6,'#E1E6EE')
icon(62,191,'search','#9AA4B4')
text(92,207,'搜索连接',13,'#9AA4B4')
icon(53,242,'chevron'); text(81,259,'生产环境',12,'#7F8A9B',600); text(246,259,'3',12,'#929CAD')
def host(y,name,ip,selected=False,live=False):
    if selected: rect(44,y-8,235,64,'#E7EEFC',7)
    icon(57,y+7,'server','#3974DC' if selected else '#8C98AA')
    text(92,y+15,name,14,'#295CAD' if selected else '#3D495C',600 if selected else 500)
    text(92,y+36,ip,12,'#8490A1')
    if live: dot(258,y+12,3.5,'#51B58A')
host(282,'生产应用 01','10.0.1.21',True,True)
host(352,'生产应用 02','10.0.1.22')
host(422,'生产数据库','10.0.1.30')
icon(53,505,'chevron'); text(81,522,'测试环境',12,'#7F8A9B',600);text(246,522,'2',12,'#929CAD')
host(546,'测试服务器','10.0.2.10',live=True)
host(616,'开发服务器','10.0.2.11')
rect(51,876,221,39,'#FFFFFF',6,'#DCE3ED');icon(103,884,'plus','#5878AA');text(132,901,'新建连接',13,'#4B6385',500)
text(136,944,'导入连接',12,'#8894A6')
# Center tabs
rect(291,127,949,46,'#F6F8FB')
rect(303,134,200,39,'#FFFFFF',6)
dot(321,154,3.5,'#4BB087');text(335,159,'生产应用 01',13,weight=600);text(480,159,'×',17,'#8B95A5')
text(526,159,'测试服务器',13,'#8992A2');text(663,159,'×',17,'#A2AAB7');icon(702,144,'plus')
line(291,173,1240,173)
text(312,204,'root @ 10.0.1.21',13,'#6F7D91',500)
rect(474,185,58,24,'#EFF8F3',4);text(485,202,'已连接',11,'#4E9A77')
icon(1138,187,'split');text(1195,204,'···',21,'#7D899C')
# Terminal
rect(291,220,949,470,'#171E29')
terminal_lines=[
('root@app-01:~# uptime','#83C5AE'),
('14:32:08 up 12 days, 3:41, 2 users, load average: 0.12, 0.18, 0.15','#D4DBE6'),
('', '#D4DBE6'),
('root@app-01:~# docker ps','#83C5AE'),
('NAMES          STATUS          PORTS','#8593A8'),
('nginx          Up 12 days      0.0.0.0:80->80/tcp','#D4DBE6'),
('app-server     Up 12 days      0.0.0.0:8080->8080/tcp','#D4DBE6'),
('redis          Up 12 days      6379/tcp','#D4DBE6'),
('', '#D4DBE6'),
('root@app-01:~# df -h /data','#83C5AE'),
('Filesystem     Size   Used   Avail   Use%   Mounted on','#8593A8'),
('/dev/vdb1      200G   128G    72G    64%   /data','#D4DBE6'),
('', '#D4DBE6'),
('root@app-01:~#','#83C5AE')]
for i,(s,c) in enumerate(terminal_lines): text(315,254+i*25,s,14,c,mono=True)
rect(438,567,8,18,'#8095B4')
rect(1231,233,3,134,'#3B4759',2)
# Files
text(313,718,'文件',13,'#3974DC',600);text(381,718,'传输',13,'#8792A4');text(450,718,'网络诊断',13,'#8792A4')
rect(311,731,29,2,'#3974DC');line(291,733,1240,733)
icon(1198,703,'chevron')
icon(311,748,'folder','#9CAABD');text(346,765,'/   var   /   log',13,'#63738A')
icon(1117,748,'refresh');icon(1160,748,'upload');icon(1203,748,'folder')
rect(292,782,947,31,'#F8F9FC')
text(320,803,'名称',11,'#9AA4B4');text(890,803,'大小',11,'#9AA4B4');text(1030,803,'修改时间',11,'#9AA4B4')
for i,(name,size,time,kind) in enumerate([('nginx','—','今天 14:20','folder'),('journal','—','今天 09:16','folder'),('syslog','2.4 MB','今天 14:32','file'),('auth.log','864 KB','今天 14:28','file')]):
    y=822+i*39
    icon(314,y,kind,'#809CC9' if kind=='folder' else '#9EACBE')
    text(347,y+16,name,13,'#56657B');text(890,y+16,size,12,'#8895A7');text(1030,y+16,time,12,'#8895A7');line(312,y+29,1219,y+29,'#F0F2F6')
# Assistant
rect(1241,127,406,857,'#FAFBFD');line(1240,127,1240,984)
icon(1264,144,'spark','#3974DC');text(1298,162,'助手',16,weight=600)
icon(1565,144,'plus');text(1610,161,'×',20,'#9AA4B4')
line(1241,173,1647,173)
icon(1264,187,'server','#96A5BB');text(1297,204,'当前连接 · 生产应用 01',12,'#8895A8')
rect(1294,245,328,52,'#EAF0FC',8)
text(1310,276,'帮我检查这台服务器的运行状态',14,'#45618B')
rect(1264,329,27,27,'#EAF0FC',7);icon(1267,332,'spark','#5B82C7');text(1303,348,'NexShell',13,weight=600)
text(1265,386,'已完成检查，服务器运行正常。',14,'#526176')
rect(1264,408,359,199,'#FFFFFF',8,'#E4EAF2')
dot(1286,433,7,'#E5F4ED');text(1281,437,'✓',11,'#4B9D79');text(1302,438,'检查完成',13,'#5A7167',500)
line(1281,452,1606,452,'#EEF1F6')
text(1282,478,'CPU 负载',13,'#8A96A7');text(1557,478,'0.12',14,'#455773',600)
text(1282,515,'运行中容器',13,'#8A96A7');text(1561,515,'3 个',14,'#455773',600)
text(1282,552,'数据盘使用率',13,'#8A96A7');text(1557,552,'64%',14,'#455773',600)
rect(1282,569,322,4,'#EEF2F8',2);rect(1282,569,206,4,'#7B9EDD',2)
text(1265,638,'暂未发现异常，磁盘空间充足。',14,'#526176')
icon(1263,668,'chevron');text(1293,685,'执行记录 · 3 条命令',12,'#8B98AA')
for i,s in enumerate(['uptime','docker ps','df -h /data']):
    text(1293,713+i*25,s,12,'#8291A6',mono=True);text(1591,713+i*25,'✓',12,'#74AE94')
rect(1259,835,371,109,'#FFFFFF',8,'#DCE3EE')
text(1276,863,'继续提问，或描述你想完成的操作…',13,'#A0ABBA')
text(1277,924,'＋',21,'#8A98AE');text(1314,922,'默认模型',12,'#8794A8');icon(1375,905,'chevron','#8794A8')
rect(1582,902,31,30,'#3974DC',6);icon(1587,906,'send','#FFFFFF')
text(1316,968,'Enter 发送 · Shift + Enter 换行',10,'#A3ACB9')
# Status and plate footer
rect(33,984,1614,31,'#FFFFFF',10);rect(33,984,1614,14,'#FFFFFF');line(33,984,1647,984)
dot(55,1000,3,'#55B38C');text(67,1004,'已连接',11,'#8794A6');text(132,1004,'延迟 24 ms',11,'#99A3B2');text(1561,1004,'2 个会话',11,'#99A3B2')
text(44,1055,'01  主工作台',13,'#61718A',600)
text(1220,1055,'视觉设计稿 · 示例数据 · 待确认',12,'#909DAF')
parts.append('</svg>')
(OUT/'nexshell-workspace-v1.svg').write_text('\n'.join(parts))
