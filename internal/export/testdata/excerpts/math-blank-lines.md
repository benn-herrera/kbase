``` math
\renewcommand{\arraystretch}{1.35}
    \begin{array}[t]{@{}l@{}}
        % horizontal arrow, overlaid above the table
        \makebox[0pt][l]{%
            \raisebox{2.2ex}[0pt][0pt]{%
                \(\scriptstyle\xrightarrow{\;\text{trivial unfolding}\;}\)
            }%
        }%
        % vertical arrow, overlaid to the left of the table
        \makebox[0pt][r]{%
            \raisebox{1.2ex}[0pt][0pt]{%
                \rotatebox[origin=lt]{-90}{%
                    \(\scriptstyle\xrightarrow{\;\text{quadratic unfolding}\;}\)
                }%
            }%
            \hspace{0.5em}%
        }%
        \begin{array}[t]{c|cc|c}
            & p=1 & p=2 & \mathop{\mathrm{Tp}}(\eta) \\
            \hline
            \ell=0
            &
            x \mapsto x^3
            &
            (x,u) \mapsto (x^3,u)
            &
s_2+    2s_{1,1}
            
            \\[1.2em]
            
            \ell=-1
            &
            (x,y) \mapsto x^3+y^2
            &
            (x,y, u) \mapsto (x^3+y^2,u)
            &
        2s_3+2s_{2,1}
            \\[1.2em]
            
            \ell=-2
            &
            (x,y,z) \mapsto x^3+y^2+z^2
            &
            (x,y,z,u) \mapsto (x^3+y^2+z^2,u)
            &
3s_{4}+     2s_{3,1}

        \end{array}
    \end{array}
```
