package gmail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/helpers"
	"github.com/EternisAI/enchanted-twin/pkg/dataprocessing/types"
)

func TestGmailProcessor(t *testing.T) {
	// Create a temporary test file
	content := `From 1828754568043628583@xxx Mon Apr 07 14:31:02 +0000 2025
X-GM-THIRD: 1828754568043628583
X-Gmail-Labels: =?UTF-8?Q?Forward_to_bob93@live.fr,Bo=C3=AEte_de_r=C3=A9ception,Non_lus?=
Delivered-To: bob@gmail.com
Received: by 2002:a05:622a:68cd:b0:471:9721:748a with SMTP id ic13csp6102107qtb;
        Mon, 7 Apr 2025 07:31:03 -0700 (PDT)
X-Google-Smtp-Source: AGHT+IGAOJR0tlrxR6lRuOwmwlCn7jmwluMHLTOSxCWRn8Ut5zK23fwrLmygD7NGQb1dougsEXjT
X-Received: by 2002:a05:6102:5f02:b0:4c5:8b0c:5fde with SMTP id ada2fe7eead31-4c854fbb75dmr8874156137.8.1744036262713;
        Mon, 07 Apr 2025 07:31:02 -0700 (PDT)
ARC-Seal: i=1; a=rsa-sha256; t=1744036262; cv=none;
        d=google.com; s=arc-20240605;
        b=C59SJvNCarMMdinFoSQ+FIHptwMrBmg83fuYdwj8GU7hDUF1koutPKqi7nuZVMwmy1
         d1MWiOMKcxK8j4L20x7diKqWLM5OG43EuXl9gMxjjhpBttptVl32ISn3jwMX8OFr0Jsf
         UXcg1D1OlZRFh1RznsSP+qm3wSebnXPasydOEoOOGrVfeeep615nRAP/dQSqYRSxwOZ/
         V8vafod/VE0zcYk3LXz+40pDu2fI5NSpIPFzVLYQJm4hoDiSp3vzwgmvLxfQxo8uiRch
         7tHcjayI13BdOXS4O4FFLv4NEpldZe/IHz2cM/v8Ue1O2w/Yl8IrxgAw1wmfPl+i63Ek
         cSFw==
ARC-Message-Signature: i=1; a=rsa-sha256; c=relaxed/relaxed; d=google.com; s=arc-20240605;
        h=from:mime-version:date:to:message-id:subject
         :content-transfer-encoding:dkim-signature;
        bh=myZz3UBCcavGyyEtepu6kC7s32D4F8HLYYYCsfDAg8E=;
        fh=a6UvMgL1J6bhXDU8hl/iBD9Sq0vWZebayHnoHLeiwiM=;
        b=g1ImzYbLRhW4YAvk76gi5AytfZic1CFHLorEkRxapBegW8JYjPt6/UxHCUNtOAScC0
         iEG8lcORjaDaeZnBvJC9LLbDAUPaGGafnrXVkgrvZ1sAHu+AkoqF02LZhW3icBdCijXj
         SHfSu+xHctT4Y4N4U/fMVNueQ5XCyzD0Cb7uaXw6yYcl8Fb3/vwq8QeA70rBLv3n6r35
         Lw3D8RcVLmHNXxk8ziZ1EheSr3LmG34noXDBS1xWTd1O5+Fe/gZsRMI2qPKf2TGl1JJN
         u4IhtuLgPVdRZt7wKI/HqgfT4hnp+aavdBDVPaOXuE7ZEJjrmaR8oDbcqwmJMR4F0SC5
         grmQ==;
        dara=google.com
ARC-Authentication-Results: i=1; mx.google.com;
       dkim=pass header.i=@meetup.com header.s=scph0624 header.b=o+33Obqh;
       spf=pass (google.com: domain of msprvs1=20192pa_4brjs=bounces-56831-18@bounces.meetup.com designates 192.174.91.231 as permitted sender) smtp.mailfrom="msprvs1=20192PA_4BrJs=bounces-56831-18@bounces.meetup.com";
       dmarc=pass (p=REJECT sp=REJECT dis=NONE) header.from=meetup.com
Return-Path: <msprvs1=20192PA_4BrJs=bounces-56831-18@bounces.meetup.com>
Received: from mta-174-91-231.evernote.com.sparkpostmail.com (mta-174-91-231.evernote.com.sparkpostmail.com. [192.174.91.231])
        by mx.google.com with ESMTPS id ada2fe7eead31-4c84904b890si2741708137.591.2025.04.07.07.31.02
        for <bob@gmail.com>
        (version=TLS1_2 cipher=ECDHE-ECDSA-AES128-GCM-SHA256 bits=128/128);
        Mon, 07 Apr 2025 07:31:02 -0700 (PDT)
Received-SPF: pass (google.com: domain of msprvs1=20192pa_4brjs=bounces-56831-18@bounces.meetup.com designates 192.174.91.231 as permitted sender) client-ip=192.174.91.231;
Authentication-Results: mx.google.com;
       dkim=pass header.i=@meetup.com header.s=scph0624 header.b=o+33Obqh;
       spf=pass (google.com: domain of msprvs1=20192pa_4brjs=bounces-56831-18@bounces.meetup.com designates 192.174.91.231 as permitted sender) smtp.mailfrom="msprvs1=20192PA_4BrJs=bounces-56831-18@bounces.meetup.com";
       dmarc=pass (p=REJECT sp=REJECT dis=NONE) header.from=meetup.com
Return-Path: <msprvs1=20192PA_4BrJs=bounces-56831-18@bounces.meetup.com>
X-MSFBL: u8l/3YWivbew+HLwYNrk/2yEZ7z6SRursb7WyBA3pyU=|eyJjdXN0b21lcl9pZCI
	6IjU2ODMxIiwibWVzc2FnZV9pZCI6IjY3ZWNhNmUxZjM2NzNjMzk0ODdkIiwidGV
	uYW50X2lkIjoic3BjIiwiciI6ImJhdGNodHJhaW5AZ21haWwuY29tIiwic3ViYWN
	jb3VudF9pZCI6IjE4In0=
DKIM-Signature: v=1; a=rsa-sha256; c=relaxed/relaxed; d=meetup.com;
	s=scph0624; t=1744036262; i=@meetup.com;
	bh=myZz3UBCcavGyyEtepu6kC7s32D4F8HLYYYCsfDAg8E=;
	h=Content-Type:Subject:Message-ID:To:Date:From:From:To:Cc:Subject;
	b=o+33Obqh3KdKmskgTcbN3BhqwmQvFfP+QvOG3IJ3JtSU8AL5Ia/TJQ5Iodf28KQG+
	 nOphdvzB4hBL/EDFMw9rKonoDpu+IUvfLpBxDpiUzkEq/fZ177HquXm32+2F55XPrn
	 jEFODs5SbnKJ/7UQtB86uO2x5DN0oFbezwkmSdsU=
Received: from [10.90.32.16] ([10.90.32.16])
	by i-0d03cca353c2e4b50.mta1vrest.sd.prd.sparkpost (ecelerity 4.8.0.74368 r(msys-ecelerity:tags/4.8.0.17)) with REST
	id D7/8C-56722-6A1E3F76; Mon, 07 Apr 2025 14:31:02 +0000
Content-Transfer-Encoding: quoted-printable
Content-Type: text/html; charset="UTF-8"
Subject: =?utf-8?B?RC5Ub3VjaCAgcG9zdGVkIGluIOKave+4j05ZQyBBZHZhbmNlZC9F?=
	=?utf-8?B?bGl0ZSBTb2NjZXIgQ29tbXVuaXR5IGJ5IEh1ZHNvbiBSaXZlciBGQw==?=
Message-ID: <D7.8C.56722.6A1E3F76@i-0d03cca353c2e4b50.mta1vrest.sd.prd.sparkpost>
To: bob@gmail.com
Date: Mon, 07 Apr 2025 14:31:02 +0000
MIME-Version: 1.0
From: "Meetup" <info@meetup.com>

<!DOCTYPE html=0A  PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://=
www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">=0A=0A<html=0A  style=3D"=
width:100%;font-family:helvetica, 'helvetica neue', arial, verdana, sans-se=
rif;-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%;padding:0;Margi=
n:0;">=0A=0A<head>=0A  <meta charset=3D"UTF-8">=0A  <meta content=3D"width=
=3Ddevice-width, initial-scale=3D1" name=3D"viewport">=0A  <meta name=3D"x-=
apple-disable-message-reformatting">=0A  <meta http-equiv=3D"X-UA-Compatibl=
e" content=3D"IE=3Dedge">=0A  <meta content=3D"telephone=3Dno" name=3D"form=
at-detection">=0A  <title>D.Touch  posted in =E2=9A=BD=EF=B8=8FNYC Advanced=
/Elite Soccer Community by Hudson River FC</title>=0A  <!--[if (mso 16)]>=
=0A  <style type=3D"text/css">=0A    a {text-decoration: none;}=0A  </style=
>=0A  <![endif]-->=0A  <!--[if gte mso 9]>=0A  <style>sup { font-size: 100%=
 !important; }</style>=0A  <![endif]-->=0A  <style type=3D"text/css" data-s=
tart-index=3D"742" data-end-index=3D"765">=0A    @font-face {=0A      font-=
family: 'Graphik Medium';=0A      src: url('https://www.meetup.com/mu_stati=
c/public/marketing/fonts/Graphik-Medium-Web.woff2') format("woff2");=0A    =
  font-style: normal;=0A      font-weight: normal;=0A    }=0A=0A    @font-f=
ace {=0A      font-family: 'Graphik Regular';=0A      src: url('https://www=
.meetup.com/mu_static/public/marketing/fonts/Graphik-Regular-Web.woff2') f=
ormat("woff2");=0A      font-style: normal;=0A      font-weight: normal;=0A=
    }=0A=0A    @font-face {=0A      font-family: 'Graphik Meetup Semi';=0A =
     src: url('https://www.meetup.com/mu_static/public/marketing/fonts/Grap=
hik-Semibold-Web.woff2') format("woff2");=0A      font-weight: 600;=0A     =
 font-style: normal;=0A      font-stretch: normal;=0A    }=0A=0A    .paddin=
g-mobile {=0A      padding-left: 50px;=0A      padding-right: 50px;=0A    }=
=0A=0A    .mobile-image {=0A      display: none;=0A    }=0A=0A    @media on=
ly screen and (max-width:600px) {=0A      h3 {=0A        font-size: 20px !i=
mportant;=0A        text-align: center;=0A        line-height: 120% !import=
ant=0A      }=0A=0A      h1 a {=0A        font-size: 30px !important=0A    =
  }=0A=0A      h2 a {=0A        font-size: 26px !important=0A      }=0A=0A =
     h3 a {=0A        font-size: 20px !important=0A      }=0A=0A      .es-m=
enu td a {=0A        font-size: 16px !important=0A      }=0A=0A      .es-he=
ader-body p,=0A      .es-header-body ul li,=0A      .es-header-body ol li,=
=0A      .es-header-body a {=0A        font-size: 16px !important=0A      }=
=0A=0A      .es-footer-body p,=0A      .es-footer-body ul li,=0A      .es-f=
ooter-body ol li,=0A      .es-footer-body a {=0A        font-size: 16px !im=
portant=0A      }=0A=0A      .es-infoblock p,=0A      .es-infoblock ul li,=
=0A      .es-infoblock ol li,=0A      .es-infoblock a {=0A        font-size=
: 12px !important=0A      }=0A=0A      *[class=3D"gmail-fix"] {=0A        d=
isplay: none !important=0A      }=0A=0A      .es-m-txt-r img,=0A      .es-m=
-txt-c img,=0A      .es-m-txt-l img {=0A        display: inline !important=
=0A      }=0A=0A      .es-button-border {=0A        display: block !importa=
nt=0A      }=0A=0A      a.es-button {=0A        display: block !important;=
=0A        border-width: 10px 0px 10px 0px !important=0A      }=0A=0A      =
.es-btn-fw {=0A        border-width: 10px 0px !important;=0A        text-a=
lign: center !important=0A      }=0A=0A      .es-adaptive table,=0A      .e=
s-btn-fw,=0A      .es-btn-fw-brdr,=0A      .es-left,=0A      .es-right {=0A=
        width: 100% !important=0A      }=0A=0A      .es-content table,=0A  =
    .es-header table,=0A      .es-footer table,=0A      .es-content,=0A    =
  .es-footer,=0A      .es-header {=0A        width: 100% !important;=0A    =
    max-width: 600px !important=0A      }=0A=0A      .es-adapt-td {=0A     =
   display: block !important;=0A        width: 100% !important=0A      }=0A=
=0A      .adapt-img-logo {=0A        width: 50% !important;=0A        heigh=
t: auto !important=0A      }=0A=0A      .adapt-img {=0A        width: 100% =
!important;=0A        height: auto !important=0A      }=0A=0A      .es-m-p0=
 {=0A        padding: 0px !important=0A      }=0A=0A      .es-m-p0r {=0A   =
     padding-right: 0px !important=0A      }=0A=0A      .es-m-p0l {=0A     =
   padding-left: 0px !important=0A      }=0A=0A      .es-m-p0t {=0A        =
padding-top: 0px !important=0A      }=0A=0A      .es-m-p0b {=0A        padd=
ing-bottom: 0 !important=0A      }=0A=0A      .es-m-p20b {=0A        paddin=
g-bottom: 20px !important=0A      }=0A=0A      .es-mobile-hidden,=0A      .=
es-hidden {=0A        display: none !important=0A      }=0A=0A      .es-des=
k-hidden {=0A        display: table-row !important;=0A        width: auto !=
important;=0A        overflow: visible !important;=0A        float: none !i=
mportant;=0A        max-height: inherit !important;=0A        line-height: =
inherit !important=0A      }=0A=0A      .es-desk-menu-hidden {=0A        di=
splay: table-cell !important=0A      }=0A=0A      table.es-table-not-adapt,=
=0A      .esd-block-html table {=0A        width: auto !important=0A      }=
=0A=0A      table.es-social {=0A        display: inline-block !important=0A=
      }=0A=0A      table.es-social td {=0A        display: inline-block !im=
portant=0A      }=0A=0A      .padding-mobile {=0A        padding-left: 16px=
;=0A        padding-right: 16px;=0A      }=0A=0A      .description-right {=
=0A        display: none !important;=0A      }=0A=0A      .mobile-image {=
=0A        display: block !important;=0A      }=0A=0A      .mobilecontent {=
=0A        display: block !important;=0A        max-height: none !important=
;=0A      }=0A=0A      div[class=3Dmobilecontent] {=0A        display: bloc=
k !important;=0A        max-height: none !important;=0A      }=0A=0A      .=
desktop-block {=0A        display: none;=0A      }=0A=0A    }=0A=0A    /*en=
d query*/=0A    #outlook a {=0A      padding: 0;=0A    }=0A=0A    .External=
Class {=0A      width: 100%;=0A    }=0A=0A    .ExternalClass,=0A    .Extern=
alClass p,=0A    .ExternalClass span,=0A    .ExternalClass font,=0A    .Ext=
ernalClass td,=0A    .ExternalClass div {=0A      line-height: 100%;=0A    =
}=0A=0A    .es-button {=0A      mso-style-priority: 100 !important;=0A     =
 text-decoration: none !important;=0A    }=0A=0A    a[x-apple-data-detector=
s] {=0A      color: inherit !important;=0A      text-decoration: none !impo=
rtant;=0A      font-size: inherit !important;=0A      font-family: inherit =
!important;=0A      font-weight: inherit !important;=0A      line-height: i=
nherit !important;=0A    }=0A=0A    .es-desk-hidden {=0A      display: none=
;=0A      float: left;=0A      overflow: hidden;=0A      width: 0;=0A      =
max-height: 0;=0A      line-height: 0;=0A      mso-hide: all;=0A    }=0A=0A=
    .date {=0A      text-transform: uppercase;=0A      color: #947F5F;=0A  =
    padding-bottom: 10px;=0A    }=0A=0A    .header-title {=0A      text-ali=
gn: center;=0A      color: #0098AB=0A    }=0A=0A    @media screen and (max-=
device-width: 480px) {=0A      div[class=3Dmobilecontent] {=0A        displ=
ay: block !important;=0A        max-height: none !important;=0A      }=0A  =
  }=0A=0A    td:empty {=0A      display: none;=0A    }=0A  </style>=0A</hea=
d>=0A=0A<body=0A  style=3D"width:100%;font-family:helvetica, 'helvetica neu=
e', arial, verdana, sans-serif;-webkit-text-size-adjust:100%;-ms-text-size-=
adjust:100%;padding:0;Margin:0;">=0D=0A<div style=3D"color:transparent;visi=
bility:hidden;opacity:0;font-size:0px;border:0;max-height:1px;width:1px;mar=
gin:0px;padding:0px;border-width:0px!important;display:none!important;line-=
height:0px!important;"><img border=3D"0" width=3D"1" height=3D"1" src=3D"ht=
tps://email-analytics.meetup.com/q/9DxKuo-igGzlN7oY_SoUNQ~~/AADd_xA~/FdlpsS=
sVr4E_kngE1SkKe7iygupIPjVtXwhXYJ6hnZaPY-oN5_m2OHkVSrFrIpf9lIo77-K-k2hnZqo95=
-e-Tw~~" alt=3D""/></div>=0D=0A=0A  <div class=3D"es-wrapper-color" style=
=3D"background-color:#F6F7F8;">=0A    <!--[if gte mso 9]><v:background xmln=
s:v=3D"urn:schemas-microsoft-com:vml" fill=3D"t"><v:fill type=3D"tile" colo=
r=3D"#f6f7f8"></v:fill></v:background><![endif]-->=0A    <table class=3D"es=
-wrapper" width=3D"100%" cellspacing=3D"0" cellpadding=3D"0"=0A      style=
=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-collapse:collapse;bord=
er-spacing:0px;padding:0;Margin:0;width:100%;height:100%;background-repeat:=
repeat;background-position:center top;">=0A      <tr style=3D"border-collap=
se:collapse;">=0A        <td valign=3D"top" style=3D"padding:0;Margin:0;">=
=0A          <table class=3D"es-content" cellspacing=3D"0" cellpadding=3D"0=
" align=3D"center"=0A            style=3D"mso-table-lspace:0pt;mso-table-rs=
pace:0pt;border-collapse:collapse;border-spacing:0px;table-layout:fixed !im=
portant;width:100%;">=0A            <tr style=3D"border-collapse:collapse;"=
>=0A              <td align=3D"center" style=3D"padding:0;Margin:0;">=0A   =
             <table class=3D"es-content-body"=0A                  style=3D"=
mso-table-lspace:0pt;mso-table-rspace:0pt;border-collapse:collapse;border-s=
pacing:0px;background-color:transparent;"=0A                  width=3D"600"=
 cellspacing=3D"0" cellpadding=3D"0" align=3D"center">=0A                  =
<tr class=3D"es-mobile-hidden" style=3D"border-collapse:collapse;">=0A     =
               <td align=3D"left" style=3D"padding:0;Margin:0;padding-top:2=
0px;padding-left:20px;padding-right:20px;">=0A                      <table =
cellpadding=3D"0" cellspacing=3D"0" width=3D"100%"=0A                      =
  style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-collapse:collap=
se;border-spacing:0px;">=0A                        <tr style=3D"border-coll=
apse:collapse;">=0A                          <td width=3D"560" align=3D"cen=
ter" valign=3D"top" style=3D"padding:0;Margin:0;">=0A                      =
      <table cellpadding=3D"0" cellspacing=3D"0" width=3D"100%"=0A         =
                     style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;bor=
der-collapse:collapse;border-spacing:0px;">=0A                             =
 <tr style=3D"border-collapse:collapse;">=0A                               =
 <td align=3D"center"=0A                                  style=3D"Margin:0=
;padding-left:20px;padding-right:20px;padding-top:30px;padding-bottom:30px;=
">=0A                                  <table border=3D"0" width=3D"100%" h=
eight=3D"100%" cellpadding=3D"0" cellspacing=3D"0"=0A                      =
              style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-col=
lapse:collapse;border-spacing:0px;">=0A                                    =
<tr style=3D"border-collapse:collapse;">=0A                                =
      <td=0A                                        style=3D"padding:0;Marg=
in:0px;border-bottom:0px solid #CCCCCC;background:none;height:1px;width:100=
%;margin:0px;">=0A                                      </td>=0A           =
                         </tr>=0A                                  </table>=
=0A                                </td>=0A                              </=
tr>=0A                            </table>=0A                          </td=
>=0A                        </tr>=0A                      </table>=0A      =
              </td>=0A                  </tr>=0A                </table>=0A=
              </td>=0A            </tr>=0A          </table>=0A          <t=
able class=3D"es-content" cellspacing=3D"0" cellpadding=3D"0" align=3D"cent=
er"=0A            style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border=
-collapse:collapse;border-spacing:0px;table-layout:fixed !important;width:1=
00%;">=0A            <tr style=3D"border-collapse:collapse;">=0A           =
   <td align=3D"center" style=3D"padding:0;Margin:0;">=0A                <t=
able class=3D"es-content-body" width=3D"600" cellspacing=3D"0" cellpadding=
=3D"0" bgcolor=3D"#ffffff"=0A                  align=3D"center"=0A         =
         style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-collapse=
:collapse;border-spacing:0px;background-color:#FFFFFF;">=0A                =
  <tr style=3D"border-collapse:collapse;">=0A                    <td align=
=3D"left"=0A                      style=3D"Margin:0;padding-top:20px;paddin=
g-bottom:20px;padding-left:20px;padding-right:20px;">=0A                   =
   <table width=3D"100%" cellspacing=3D"0" cellpadding=3D"0"=0A            =
            style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-colla=
pse:collapse;border-spacing:0px;">=0A                        <tr style=3D"b=
order-collapse:collapse;">=0A                          <td width=3D"560" va=
lign=3D"top" align=3D"center" style=3D"padding:0;Margin:0;">=0A            =
                <table width=3D"100%" cellspacing=3D"0" cellpadding=3D"0"=
=0A                              style=3D"mso-table-lspace:0pt;mso-table-rs=
pace:0pt;border-collapse:collapse;border-spacing:0px;">=0A                 =
             <tr style=3D"border-collapse:collapse;">=0A                   =
             <td align=3D"left" class=3D"es-m-txt-l" style=3D"padding:0;Mar=
gin:0;padding-left:5px;"><img=0A                                    src=3D"=
https://adigj.stripocdn.email/content/guids/CABINET_ccee1027ec1cc651cb3e28a=
2ae4c2b08/images/28571562104029468.png"=0A                                 =
   alt=3D"meetup logo"=0A                                    style=3D"displ=
ay:block;border:0;outline:none;text-decoration:none;-ms-interpolation-mode:=
bicubic;"=0A                                    width=3D"100"></td>=0A     =
                         </tr>=0A                              <tr style=3D=
"border-collapse:collapse;">=0A                                <td align=3D=
"left"=0A                                  style=3D"padding:0;Margin:0;padd=
ing-left:10px;padding-right:10px;padding-top:20px;">=0A                    =
              <p=0A                                    style=3D"Margin:0;-w=
ebkit-text-size-adjust:none;-ms-text-size-adjust:none;mso-line-height-rule:=
exactly;font-size:21px;font-family:helvetica, 'helvetica neue', arial, verd=
ana, sans-serif;line-height:30px;color:#212121;">=0A                       =
             <strong>D.Touch  started a discussion in <a href=3D"https://ww=
w.meetup.com/new-york-pick-up-soccer-group/=3Futm_medium=3Demail&utm_campai=
gn=3Dgroup-discussion-announce">=E2=9A=BD=EF=B8=8FNYC Advanced/Elite Soccer=
 Community by Hudson River FC</a>.</strong>=0A                             =
     </p>=0A                                  <p=0A                        =
            style=3D"Margin:0;-webkit-text-size-adjust:none;-ms-text-size-a=
djust:none;mso-line-height-rule:exactly;font-size:17px;font-family:helvetic=
a, 'helvetica neue', arial, verdana, sans-serif;line-height:26px;color:#212=
121;">=0A                                    <br>=0A                       =
           </p>=0A                                                         =
             <a href=3D"https://www.meetup.com/members/470391853" style=3D"=
display:block;border:none;" border=3D"0">=0A                               =
       <img style=3D"vertical-align:middle;border-radius:999px;object-fit:c=
over;background-color:#EEEEEE;" src=3D"https://secure.meetupstatic.com/phot=
os/member/9/e/0/e/thumb_322600462.jpeg" alt=3D"D.Touch " height=3D"48" widt=
h=3D"48" />=0A                                    </a>=0A                  =
                                                  <p=0A                    =
                style=3D"Margin:0;-webkit-text-size-adjust:none;-ms-text-si=
ze-adjust:none;mso-line-height-rule:exactly;font-size:17px;font-family:helv=
etica, 'helvetica neue', arial, verdana, sans-serif;line-height:26px;color:=
#212121;">=0A                                    <strong>D.Touch </strong>=
=0A                                  </p>=0A                               =
   <p=0A                                    style=3D"Margin:0;-webkit-text-=
size-adjust:none;-ms-text-size-adjust:none;mso-line-height-rule:exactly;fon=
t-size:17px;font-family:helvetica, 'helvetica neue', arial, verdana, sans-s=
erif;line-height:26px;color:#212121;">=0A                                  =
  I want to give out my MacBook Air 2023 for free it=E2=80=99s in health an=
d good condition along side a charger so it=E2=80=99s perfect , I want to g=
ive it because I just got a new one so I want to give it out to anyone inte=
rested in it you can text me on 310-421-4920=0A                            =
      </p>=0A                                </td>=0A                      =
        </tr>=0A                              <tr style=3D"border-collapse:=
collapse;">=0A                                <td align=3D"left"=0A        =
                          style=3D"Margin:0;padding-top:10px;padding-bottom=
:10px;padding-left:10px;padding-right:10px;">=0A                           =
       <span class=3D"es-button-border"=0A                                 =
   style=3D"mso-style-priority:100 !important;text-decoration:none;-webkit-=
text-size-adjust:none;-ms-text-size-adjust:none;mso-line-height-rule:exactl=
y;font-family:helvetica, 'helvetica neue', arial, verdana, sans-serif;font-=
size:16px;color:#FFFFFF;border-style:solid;border-color:#F65858;border-widt=
h:10px 20px;display:inline-block;background:#F65858;border-radius:10px;font=
-weight:normal;font-style:normal;line-height:19px;width:auto;text-align:cen=
ter;">=0A                                    <a href=3D"https://www.meetup.=
com/new-york-pick-up-soccer-group//discussions/6755397672930411/=3Futm_medi=
um=3Demail&utm_campaign=3Dgroup-discussion-announce" class=3D"es-button" ta=
rget=3D"_blank"=0A                                      style=3D"color:#fff=
;">View discussion</a>=0A                                  </span></td>=0A =
                             </tr>=0A                              <tr styl=
e=3D"border-collapse:collapse;">=0A                                <td alig=
n=3D"center"=0A                                  style=3D"Margin:0;padding-=
top:10px;padding-bottom:10px;padding-left:20px;padding-right:20px;">=0A    =
                              <table border=3D"0" width=3D"100%" height=3D"=
100%" cellpadding=3D"0" cellspacing=3D"0"=0A                               =
     style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-collapse:col=
lapse;border-spacing:0px;">=0A                                    <tr style=
=3D"border-collapse:collapse;">=0A                                      <td=
=0A                                        style=3D"padding:0;Margin:0px;bo=
rder-bottom:0px solid #CCCCCC;background:none;height:1px;width:100%;margin:=
0px;">=0A                                      </td>=0A                    =
                </tr>=0A                                  </table>=0A      =
                          </td>=0A                              </tr>=0A   =
                         </table>=0A                          </td>=0A     =
                   </tr>=0A                      </table>=0A               =
     </td>=0A                  </tr>=0A                </table>=0A         =
     </td>=0A            </tr>=0A          </table>=0A          <table class=
s=3D"es-footer" cellspacing=3D"0" cellpadding=3D"0" align=3D"center"=0A    =
        style=3D"mso-table-lspace:0pt;mso-table-rspace:0pt;border-collapse:=
collapse;border-spacing:0px;table-layout:fixed !important;width:100%;backgr=
ound-color:transparent;background-repeat:repeat;background-position:center =
top;">=0A            <tr style=3D"border-collapse:collapse;">=0A           =
   <td align=3D"center" style=3D"padding:0;Margin:0;">=0A                <t=
able class=3D"es-footer-body" width=3D"600" cellspacing=3D"0" cellpadding=
=3D"0" align=3D"center"=0A                  style=3D"mso-table-lspace:0pt;m=
so-table-rspace:0pt;border-collapse:collapse;border-spacing:0px;background-=
color:transparent;">=0A                  <tr style=3D"border-collapse:colla=
pse;">=0A                    <td align=3D"left" style=3D"padding:20px;Margi=
n:0;">=0A                      <table width=3D"100%" cellspacing=3D"0" cell=
padding=3D"0"=0A                        style=3D"mso-table-lspace:0pt;mso-t=
able-rspace:0pt;border-collapse:collapse;border-spacing:0px;">=0A          =
              <tr style=3D"border-collapse:collapse;">=0A                  =
        <td width=3D"560" valign=3D"top" align=3D"center" style=3D"padding:=
0;Margin:0;">=0A                            <table width=3D"100%" cellspaci=
ng=3D"0" cellpadding=3D"0"=0A                              style=3D"mso-tab=
le-lspace:0pt;mso-table-rspace:0pt;border-collapse:collapse;border-spacing:=
0px;">=0A                              <tr style=3D"border-collapse:collaps=
e;">=0A                                <td esdev-links-color=3D"#666666" al=
ign=3D"center"=0A                                  style=3D"padding:0;Margi=
n:0;padding-bottom:10px;">=0A                                  <p=0A       =
                             style=3D"Margin:0;-webkit-text-size-adjust:non=
e;-ms-text-size-adjust:none;mso-line-height-rule:exactly;font-size:11px;fon=
t-family:helvetica, 'helvetica neue', arial, verdana, sans-serif;line-heigh=
t:17px;color:#666666;">=0A                                    <br>=0A      =
                            </p>=0A                                  <p=0A =
                                   style=3D"Margin:0;-webkit-text-size-adju=
st:none;-ms-text-size-adjust:none;mso-line-height-rule:exactly;font-size:11=
px;font-family:helvetica, 'helvetica neue', arial, verdana, sans-serif;line=
-height:17px;color:#666666;">=0A                                    <a targ=
et=3D"_blank"=0A                                      style=3D"-webkit-text=
-size-adjust:none;-ms-text-size-adjust:none;mso-line-height-rule:exactly;fo=
nt-family:helvetica, 'helvetica neue', arial, verdana, sans-serif;font-size=
:11px;text-decoration:underline;color:#666666;"=0A                         =
             href=3D"https://www.meetup.com/email-unsubscribe/=3Fmid=3D2852=
67627&code=3Dconversation_announce&group_id=3D37084276&eo=3Dmca1&expires=3D=
1746628256800&sig=3De9c05a68229c05163b5009286170bd390908170b">Unsubscribe</=
a> from these type of emails.=0A                                    </p>=0A=
                                  <p=0A                                    =
style=3D"Margin:0;-webkit-text-size-adjust:none;-ms-text-size-adjust:none;m=
so-line-height-rule:exactly;font-size:11px;font-family:helvetica, 'helvetic=
a neue', arial, verdana, sans-serif;line-height:17px;color:#666666;">=0A   =
                                 Manage your <a target=3D"_blank"=0A       =
                               style=3D"-webkit-text-size-adjust:none;-ms-t=
ext-size-adjust:none;mso-line-height-rule:exactly;font-family:helvetica, 'h=
elvetica neue', arial, verdana, sans-serif;font-size:11px;text-decoration:u=
nderline;color:#666666;"=0A                                      href=3D"ht=
tps://www.meetup.com/account/comm/">Email Notification Preferences</a>.=0A =
                                 </p>=0A                                  <=
p=0A                                    style=3D"Margin:0;-webkit-text-size=
-adjust:none;-ms-text-size-adjust:none;mso-line-height-rule:exactly;font-si=
ze:11px;font-family:helvetica, 'helvetica neue', arial, verdana, sans-serif=
;line-height:17px;color:#666666;">=0A                                    Re=
ad our <a target=3D"_blank"=0A                                      style=
=3D"-webkit-text-size-adjust:none;-ms-text-size-adjust:none;mso-line-height=
-rule:exactly;font-family:helvetica, 'helvetica neue', arial, verdana, sans=
-serif;font-size:11px;text-decoration:underline;color:#666666;"=0A         =
                             href=3D"https://www.meetup.com/privacy">Privac=
y=0A                                      Policy</a>.=0A                   =
               </p>=0A                                </td>=0A             =
                 </tr>=0A                              <tr style=3D"border-=
collapse:collapse;">=0A                                <td esdev-links-colo=
r=3D"#666666" align=3D"center"=0A                                  style=3D=
"padding:0;Margin:0;padding-bottom:10px;">=0A                              =
    <p=0A                                    style=3D"Margin:0;-webkit-text=
-size-adjust:none;-ms-text-size-adjust:none;mso-line-height-rule:exactly;fo=
nt-size:11px;font-family:helvetica, 'helvetica neue', arial, verdana, sans-=
serif;line-height:17px;color:#666666;">=0A                                 =
   Meetup LLC, 169 Madison Ave., STE 11218, NY, NY 10016</p>=0A            =
                    </td>=0A                              </tr>=0A         =
                   </table>=0A                          </td>=0A           =
             </tr>=0A                      </table>=0A                    <=
/td>=0A                  </tr>=0A                </table>=0A              <=
/td>=0A            </tr>=0A          </table>=0A        </td>=0A      </tr>=
=0A    </table>=0A  </div>=0A=0D=0A<img border=3D"0" width=3D"1" height=3D"=
1" alt=3D"" src=3D"https://email-analytics.meetup.com/q/eoIntyqGGgboJCu2OL4=
xMw~~/AADd_xA~/QbiY05sPxkwluqnHhHM2xigXWrk-IA51qQbTEugcKaz5pzj4TL_-vyzxYvGU=
aifHP9gTjCs0OGaIqGiLqUHisw~~">=0D=0A</body>=0A=0A=0A</html>=0A

`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.mbox")
	err := os.WriteFile(tmpFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create Gmail processor
	processor := NewGmailProcessor()

	// Test processor name
	if processor.Name() != "gmail" {
		t.Errorf("Expected processor name to be 'gmail', got %s", processor.Name())
	}

	// Process the test file
	records, err := processor.ProcessFile(tmpFile, "bob@gmail.com")
	if err != nil {
		t.Fatalf("Failed to process file: %v", err)
	}

	// Verify we got one record
	assert.Equal(t, 1, len(records), "Expected 1 record")

	record := records[0]

	// Test record fields
	expectedTime, _ := time.Parse(time.RFC1123Z, "Mon, 07 Apr 2025 14:31:02 +0000")
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Source", record.Source, "gmail"},
		{"From", record.Data["from"], "\"Meetup\" <info@meetup.com>"},
		{"To", record.Data["to"], "bob@gmail.com"},
		{"Timestamp", record.Timestamp.UTC(), expectedTime.UTC()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !assert.Equal(t, tt.expected, tt.got) {
				t.Errorf("Expected %v, got %v", tt.expected, tt.got)
			}
		})
	}

	// Check that content is not empty and correctly cleaned
	content, ok := record.Data["content"].(string)
	assert.True(t, ok, "Content should be a string")
	assert.NotEmpty(t, content, "Content should not be empty after cleaning")
	// Add more specific checks:
	assert.Contains(t, content, "D.Touch", "Cleaned content should contain part of the main body")
	assert.NotContains(t, content, "https://", "Cleaned content should not contain https links")
	assert.NotContains(t, content, "http://", "Cleaned content should not contain http links") // Also check http
	assert.NotContains(t, content, "Unsubscribe", "Cleaned content should not contain footer elements like 'Unsubscribe'")
	assert.NotContains(t, content, "Privacy Policy", "Cleaned content should not contain footer elements like 'Privacy Policy'")
}

func TestToDocuments(t *testing.T) {
	// Create a temporary test file with anonymized sample data
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.jsonl")

	// Anonymized sample data based on the provided email
	sampleData := `{"data":{"content":"Welcome aboard, testuser! Here are some tips to get you started\n\n---\n\nSign in\n===\nhttps://example.com/auth/sign_in\n\n---\n\nWelcome Checklist\n===\nLet's get you started on this new social frontier:\n\n1. Personalize your profile\n   Boost your interactions by having a comprehensive profile.\n   * https://example.com/web/start/profile\n\n2. Personalize your home feed\n   Following interesting people is what this platform is all about.\n   * https://example.com/web/start/follows\n\n3. Make your first post\n   Say hello to the world with text, photos, videos, or polls.\n   * https://example.com/web\n\n4. Share your profile\n   Let your friends know how to find you.\n   * https://example.com/web/start/share\n\n5. Download apps\n   Download our official apps.\n   * iOS: https://example.com/ios\n   * Android: https://example.com/android\n\n---\n\nWho to follow\n===\nFollow well-known accounts\n\n* user1 · @user1\n  https://example.com/web/@user1\n* user2 · @user2\n  https://example.com/web/@user2\n\nhttps://example.com/web/explore/suggestions\n\n---\n\nTrending hashtags\n===\nExplore what's trending since past 2 days\n\nhttps://example.com/web/explore/tags\n\n---\n\nStay in control of your own timeline\n===\nYou know best what you want to see on your home feed. No algorithms or ads to\nwaste your time. Follow anyone across any server from a single account\nand receive their posts in chronological order, and make your corner of the\ninternet a little more like you.\n\nBuild your audience in confidence\n===\nThis platform provides you with a unique possibility of managing your audience\nwithout middlemen. Deployed on your own infrastructure allows you to\nfollow and be followed from any other server online and is under no\none's control but yours.\n\nModerating the way it should be\n===\nThis platform puts decision making back in your hands. Each server creates their own\nrules and regulations, which are enforced locally and not top-down like\ncorporate social media, making it the most flexible in responding to the needs\nof different groups of people. Join a server with the rules you agree with, or\nhost your own.\n\nUnparalleled creativity\n===\nThis platform supports audio, video and picture posts, accessibility descriptions,\npolls, content warnings, animated avatars, custom emojis, thumbnail crop\ncontrol, and more, to help you express yourself online. Whether you're\npublishing your art, your music, or your podcast, this platform is there for you.\n\n---\n\nPlatform hosted on example.com\nChange email preferences: https://example.com/settings/preferences","from":"support@example.com","myMessage":false,"subject":"Welcome to the Platform","to":"testuser@example.com"},"timestamp":"2025-02-25T09:46:33Z","source":"gmail"}`

	err := os.WriteFile(tmpFile, []byte(sampleData), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test ToDocuments function
	records, err := helpers.ReadJSONL[types.Record](tmpFile)
	if err != nil {
		t.Fatalf("Failed to convert to documents: %v", err)
	}
	gmailProcessor := NewGmailProcessor()
	documents, err := gmailProcessor.ToDocuments(records)
	if err != nil {
		t.Fatalf("Failed to convert to documents: %v", err)
	}

	// Verify we got one document
	if len(documents) != 1 {
		t.Fatalf("Expected 1 document, got %d", len(documents))
	}

	doc := documents[0]

	// Test document fields
	expectedTime, _ := time.Parse(time.RFC3339, "2025-02-25T09:46:33Z")

	// Test content separately since it's a string comparison
	if !strings.Contains(doc.Content(), "Welcome aboard, testuser!") {
		t.Errorf("Expected content to contain 'Welcome aboard, testuser!'")
	}

	// Test other fields
	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Timestamp", doc.Timestamp().UTC(), expectedTime.UTC()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tt.got)
			}
		})
	}

	// Test Tags
	expectedTags := []string{"google", "email"}
	if len(doc.Tags()) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d", len(expectedTags), len(doc.Tags()))
	}
	for _, tag := range expectedTags {
		found := false
		for _, gotTag := range doc.Tags() {
			if gotTag == tag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tag '%s' not found in document tags", tag)
		}
	}

	// Test Metadata
	expectedMetadata := map[string]string{
		"from":    "support@example.com",
		"to":      "testuser@example.com",
		"subject": "Welcome to the Platform",
	}
	if len(doc.Metadata()) != len(expectedMetadata) {
		t.Errorf("Expected %d metadata entries, got %d", len(expectedMetadata), len(doc.Metadata()))
	}
	for key, value := range expectedMetadata {
		if gotValue, ok := doc.Metadata()[key]; !ok {
			t.Errorf("Expected metadata key '%s' not found", key)
		} else if gotValue != value {
			t.Errorf("For metadata key '%s', expected value '%s', got '%s'", key, value, gotValue)
		}
	}
}

func TestCleanEmailText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove links and footer",
			input:    "Hello,\n\nCheck out this link: https://example.com/page1\nAnd this one: http://another-site.org/path?query=1\n\nSome normal text.\n\n--\nUnsubscribe here: https://unsubscribe.link\nRead our privacy policy https://policy.com\n© 2024 Company Name",
			expected: "Hello,\nCheck out this link:\nAnd this one:\nSome normal text.\n--", // Footer lines and empty lines are removed
		},
		{
			name:     "only links",
			input:    `This has a link https://link.com/test but no footer.`,
			expected: `This has a link  but no footer.`,
		},
		{
			name: "only footer",
			input: `This has no link.

unsubscribe please
This email was sent to you.`,
			expected: `This has no link.`,
		},
		{
			name: "no links or footer",
			input: `Just plain text.
With multiple lines.`,
			expected: `Just plain text.
With multiple lines.`,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "user provided example snippet",
			input:    "👋 Hello, Retool community! There's never been a better time to join our Community Forum (https://cWGb504.na1.hubspotlinks.com/Ctc/LY+113/cWGb504/VVVRyG8Z8BFlW1dz0y_47-q3tVGwS3m5w0KyMN2QYvzl3lcq-W69sMD-6lZ3ndW9jjRzn47xHM7W8r-xrj13QL0YW5pflwb763qLfW3PC8073ZyrDKV_3w_S9d3V5zW1kHLSH3mx5f2W3-yQXd5cFRX8W4qbYlk7v96bxW7t6f9129jTNqW2YdNC23Vxs0sW2yJMyH6zmm62W783_Ny14QXTQW3HF_pZ6PTgpjW8ms3S05r-QQ8W6Yjz2_2C49_PN1J05Jl9ZQWnV6zFll8rvVqsW1nbvBV6xqd5XW9bdDXF1_V2HqW1qCZwP46G_Rbf2jQ0bx04 ) . We just launched Builder Talks—a new live, AMA-style series featuring real Retool builders sharing their journeys.",
			expected: "👋 Hello, Retool community! There's never been a better time to join our Community Forum ( ) . We just launched Builder Talks—a new live, AMA-style series featuring real Retool builders sharing their journeys.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := cleanEmailText(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
